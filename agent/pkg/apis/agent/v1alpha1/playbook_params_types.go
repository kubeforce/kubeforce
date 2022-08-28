package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:conversion-gen:explicit-from=net/url.Values
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlaybookLogOptions is the query options for a playbook's logs REST call.
type PlaybookLogOptions struct {
	metav1.TypeMeta `json:",inline"`
	// Follow the log stream of the playbook. Defaults to false.
	// +optional
	Follow bool `json:"follow,omitempty" protobuf:"varint,1,opt,name=follow"`
	// Return previous terminated container logs. Defaults to false.
	// +optional
	Previous bool `json:"previous,omitempty" protobuf:"varint,2,opt,name=previous"`
	// A relative time in seconds before the current time from which to show logs. If this value
	// precedes the time a playbook was started, only logs since the playbook start will be returned.
	// If this value is in the future, no logs will be returned.
	// Only one of sinceSeconds or sinceTime may be specified.
	// +optional
	SinceSeconds *int64 `json:"sinceSeconds,omitempty" protobuf:"varint,3,opt,name=sinceSeconds"`
	// An RFC3339 timestamp from which to show logs. If this value
	// precedes the time a playbook was started, only logs since the playbook start will be returned.
	// If this value is in the future, no logs will be returned.
	// Only one of sinceSeconds or sinceTime may be specified.
	// +optional
	SinceTime *metav1.Time `json:"sinceTime,omitempty" protobuf:"bytes,4,opt,name=sinceTime"`
	// If true, add an RFC3339 or RFC3339Nano timestamp at the beginning of every line
	// of log output. Defaults to false.
	// +optional
	Timestamps bool `json:"timestamps,omitempty" protobuf:"varint,5,opt,name=timestamps"`
	// If set, the number of lines from the end of the logs to show. If not specified,
	// logs are shown from the creation of the container or sinceSeconds or sinceTime
	// +optional
	TailLines *int64 `json:"tailLines,omitempty" protobuf:"varint,6,opt,name=tailLines"`
}
