package apiserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	apisinstall "k3f.io/kubeforce/agent/pkg/apis/agent/install"
	"k3f.io/kubeforce/agent/pkg/apis/agent/v1alpha1"
	"k3f.io/kubeforce/agent/pkg/config"
	generatedopenapi "k3f.io/kubeforce/agent/pkg/generated/openapi"
	"k3f.io/kubeforce/agent/pkg/install"
	agentrest "k3f.io/kubeforce/agent/pkg/registry/agent/rest"
	"k3f.io/kubeforce/agent/pkg/registry/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/features"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/server"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/options"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	clientgoinformers "k8s.io/client-go/informers"
	clientgoclientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	certutil "k8s.io/client-go/util/cert"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"
	openapicommon "k8s.io/kube-openapi/pkg/common"
)

var (
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme = runtime.NewScheme()
	// Codecs provides methods for retrieving codecs and serializers for specific
	// versions and content types.
	Codecs = serializer.NewCodecFactory(Scheme)

	ParameterCodec = runtime.NewParameterCodec(Scheme)
)

func NewServer(cfg config.ConfigSpec) (*Server, error) {
	s := &Server{
		config:  cfg,
		started: make(chan struct{}),
	}
	if err := s.init(); err != nil {
		return nil, err
	}
	return s, nil
}

type Server struct {
	config             config.ConfigSpec
	genericAPIServer   *genericapiserver.GenericAPIServer
	VersionedInformers clientgoinformers.SharedInformerFactory
	// LoopbackClientConfig is a config for a privileged loopback connection to the API server
	LoopbackClientConfig *restclient.Config
	completedConfig      *genericapiserver.CompletedConfig
	started              chan struct{}
}

// InstallAPIs will install the APIs for the restStorageProviders if they are enabled.
func (s *Server) InstallAPIs(restOptionsGetter generic.RESTOptionsGetter, restStorageProviders ...storage.RESTStorageProvider) error {
	var apiGroupsInfo []*genericapiserver.APIGroupInfo

	req := &storage.RESTStorageRequest{
		Scheme:            Scheme,
		ParameterCodec:    ParameterCodec,
		Codecs:            Codecs,
		RestOptionsGetter: restOptionsGetter,
		Config:            s.config,
	}
	for _, restStorageBuilder := range restStorageProviders {
		groupName := restStorageBuilder.GroupName()
		apiGroupInfo, err := restStorageBuilder.NewRESTStorage(req)
		if err != nil {
			return fmt.Errorf("problem initializing API group %q : %v", groupName, err)
		}

		klog.V(1).Infof("Enabling API group %q.", groupName)

		if postHookProvider, ok := restStorageBuilder.(genericapiserver.PostStartHookProvider); ok {
			name, hook, err := postHookProvider.PostStartHook()
			if err != nil {
				klog.Fatalf("Error building PostStartHook: %v", err)
			}
			s.genericAPIServer.AddPostStartHookOrDie(name, hook)
		}

		apiGroupsInfo = append(apiGroupsInfo, &apiGroupInfo)
	}

	if err := s.genericAPIServer.InstallAPIGroups(apiGroupsInfo...); err != nil {
		return fmt.Errorf("error in registering group versions: %v", err)
	}
	return nil
}

func (s *Server) init() error {
	apisinstall.Install(Scheme)
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})
	utilruntime.Must(metav1.AddMetaToScheme(Scheme))

	gvs := []schema.GroupVersion{v1alpha1.SchemeGroupVersion}
	recommendedOptions := genericoptions.NewRecommendedOptions(
		"/registry/kubeforce-apiserver",
		Codecs.LegacyCodec(gvs...),
	)
	etcdServers := s.config.Etcd.ListenClientURLs
	recommendedOptions.Etcd.StorageConfig.Transport.ServerList = strings.Split(etcdServers, ",")
	recommendedOptions.Etcd.StorageConfig.Transport.KeyFile = keyFilePath(s.config.Etcd.CertsDir, etcdClientBaseName)
	recommendedOptions.Etcd.StorageConfig.Transport.CertFile = certFilePath(s.config.Etcd.CertsDir, etcdClientBaseName)
	recommendedOptions.Etcd.StorageConfig.Transport.TrustedCAFile = certFilePath(s.config.Etcd.CertsDir, etcdCaBaseName)
	recommendedOptions.Authentication = nil
	recommendedOptions.Authorization = nil
	recommendedOptions.CoreAPI = nil
	recommendedOptions.Admission = nil
	recommendedOptions.SecureServing = &options.SecureServingOptionsWithLoopback{}
	recommendedOptions.Etcd.StorageConfig.Paging = utilfeature.DefaultFeatureGate.Enabled(features.APIListChunking)

	if err := utilerrors.NewAggregate(recommendedOptions.Validate()); err != nil {
		return err
	}

	serverConfig := genericapiserver.NewRecommendedConfig(Codecs)

	if err := recommendedOptions.ApplyTo(serverConfig); err != nil {
		return err
	}
	var err error
	if serverConfig.SecureServing, err = createSecureServing(s.config); err != nil {
		return err
	}
	if serverConfig.LoopbackClientConfig, err = createLoopBackConfig(serverConfig.SecureServing); err != nil {
		return err
	}

	if err := applyToAuthentication(&serverConfig.Authentication, serverConfig.SecureServing, serverConfig.OpenAPIConfig, s.config); err != nil {
		return err
	}

	completedConfig := serverConfig.Complete()
	agentVersion := version.Get()

	if agentVersion.Major == "" && agentVersion.Minor == "" {
		agentVersion.Major = "0"
		agentVersion.Minor = "1"
	}
	completedConfig.Version = &agentVersion

	completedConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(Scheme))
	completedConfig.OpenAPIConfig.Info.Title = "kubeforce-agent"
	completedConfig.OpenAPIConfig.Info.Version = agentVersion.GitVersion
	completedConfig.LongRunningFunc = filters.BasicLongRunningRequestCheck(
		sets.NewString("watch"),
		sets.NewString("exec", "log"),
	)

	// Disable compression for self-communication, since we are going to be
	// on a fast local network
	completedConfig.LoopbackClientConfig.DisableCompression = true

	kubeClientConfig := completedConfig.LoopbackClientConfig
	s.LoopbackClientConfig = kubeClientConfig
	clientgoExternalClient, err := clientgoclientset.NewForConfig(kubeClientConfig)
	if err != nil {
		return fmt.Errorf("failed to create real external clientset: %v", err)
	}
	s.VersionedInformers = clientgoinformers.NewSharedInformerFactory(clientgoExternalClient, 10*time.Minute)
	s.completedConfig = &completedConfig
	genericServer, err := s.completedConfig.New("sample-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return err
	}
	s.genericAPIServer = genericServer
	s.InstallDefaultHandlers()
	genericServer.AddPostStartHookOrDie("ready", s.readyHook())
	genericServer.ShutdownTimeout = s.config.ShutdownGracePeriod.Duration
	return nil
}

// InstallDefaultHandlers registers the default set of supported HTTP request
func (s *Server) InstallDefaultHandlers() {
	klog.InfoS("Adding default handlers to agent server")
	s.genericAPIServer.Handler.NonGoRestfulMux.HandleFunc("/uninstall", s.uninstall)
}

// createSecureServing fills up serving information in the server configuration.
func createSecureServing(cfg config.ConfigSpec) (*server.SecureServingInfo, error) {
	if cfg.Port <= 0 {
		return nil, fmt.Errorf("unable to use port: %d", cfg.Port)
	}

	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(cfg.Port))
	lc := net.ListenConfig{}
	listener, _, err := options.CreateListener("tcp", addr, lc)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %v", err)
	}

	c := &server.SecureServingInfo{
		Listener: listener,
	}

	if len(cfg.TLS.CertData) != 0 || len(cfg.TLS.PrivateKeyData) != 0 {
		c.Cert, err = dynamiccertificates.NewStaticCertKeyContent("serving-cert", cfg.TLS.CertData, cfg.TLS.PrivateKeyData)
		if err != nil {
			return nil, err
		}
	} else if len(cfg.TLS.CertFile) != 0 || len(cfg.TLS.PrivateKeyFile) != 0 {
		c.Cert, err = dynamiccertificates.NewDynamicServingContentFromFiles("serving-cert", cfg.TLS.CertFile, cfg.TLS.PrivateKeyFile)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("tls certificate and private key is not defined")
	}

	if len(cfg.TLS.CipherSuites) != 0 {
		cipherSuites, err := cliflag.TLSCipherSuites(cfg.TLS.CipherSuites)
		if err != nil {
			return nil, err
		}
		c.CipherSuites = cipherSuites
	}

	if len(cfg.TLS.TLSMinVersion) != 0 {
		minTLSVersion, err := cliflag.TLSVersion(cfg.TLS.TLSMinVersion)
		if err != nil {
			return nil, errors.Wrapf(err, "use tls version from https://golang.org/pkg/crypto/tls/#pkg-constants")
		}
		c.MinTLSVersion = minTLSVersion
	}

	c.SNICerts = make([]dynamiccertificates.SNICertKeyContentProvider, 0)

	return c, nil
}

func createLoopBackConfig(secureServingInfo *genericapiserver.SecureServingInfo) (*restclient.Config, error) {
	// create self-signed cert+key with the fake server.LoopbackClientServerNameOverride and
	// let the server return it when the loopback client connects.
	certPem, keyPem, err := certutil.GenerateSelfSignedCertKey(server.LoopbackClientServerNameOverride, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate self-signed certificate for loopback connection: %v", err)
	}
	certProvider, err := dynamiccertificates.NewStaticSNICertKeyContent("self-signed loopback", certPem, keyPem, server.LoopbackClientServerNameOverride)
	if err != nil {
		return nil, fmt.Errorf("failed to generate self-signed certificate for loopback connection: %v", err)
	}

	// Write to the front of SNICerts so that this overrides any other certs with the same name
	secureServingInfo.SNICerts = append([]dynamiccertificates.SNICertKeyContentProvider{certProvider}, secureServingInfo.SNICerts...)

	return secureServingInfo.NewLoopbackClientConfig(uuid.New().String(), certPem)
}

func applyToAuthentication(authenticationInfo *server.AuthenticationInfo, servingInfo *server.SecureServingInfo, openAPIConfig *openapicommon.Config, cfg config.ConfigSpec) error {
	authCfg := authenticatorfactory.DelegatingAuthenticatorConfig{
		Anonymous: false,
	}

	if len(cfg.Authentication.X509.ClientCAData) > 0 {
		cert, err := dynamiccertificates.NewStaticCAContent("client-ca", cfg.Authentication.X509.ClientCAData)
		if err != nil {
			return err
		}
		authCfg.ClientCertificateCAContentProvider = cert
	} else if len(cfg.Authentication.X509.ClientCAFile) > 0 {
		cert, err := dynamiccertificates.NewDynamicCAContentFromFile("client-ca", cfg.Authentication.X509.ClientCAFile)
		if err != nil {
			return err
		}
		authCfg.ClientCertificateCAContentProvider = cert
	} else {
		return errors.New("authentication is not configured")
	}
	servingInfo.ClientCA = authCfg.ClientCertificateCAContentProvider

	// create authenticator
	authenticator, securityDefinitions, err := authCfg.New()
	if err != nil {
		return err
	}
	authenticationInfo.Authenticator = authenticator
	if openAPIConfig != nil {
		openAPIConfig.SecurityDefinitions = securityDefinitions
	}

	return nil
}

func (s *Server) readyHook() genericapiserver.PostStartHookFunc {
	return func(hookCtx genericapiserver.PostStartHookContext) error {
		close(s.started)
		return nil
	}
}

func (s *Server) ReadyNotify() <-chan struct{} {
	return s.started
}

func (s *Server) Start(ctx context.Context) error {
	restStorageProviders := []storage.RESTStorageProvider{
		agentrest.StorageProvider{},
	}

	if err := s.InstallAPIs(s.completedConfig.RESTOptionsGetter, restStorageProviders...); err != nil {
		return err
	}

	apiServer := s.genericAPIServer.PrepareRun()
	return apiServer.Run(ctx.Done())
}

func (s *Server) uninstall(resp http.ResponseWriter, req *http.Request) {
	if err := install.Uninstall(req.Context(), &s.config, false); err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		responsewriters.ErrorNegotiated(
			apierrors.NewInternalError(err),
			Codecs, schema.GroupVersion{}, resp, req,
		)
		return
	}
	resp.WriteHeader(http.StatusOK)
}
