/*
Copyright 2021 The Kubeforce Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudinit

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
)

const cloudData = `
## template: jinja
#cloud-config

write_files:
-   path: /etc/kubernetes/pki/ca.crt
    owner: root:root
    permissions: '0640'
    content: |
      -----BEGIN CERTIFICATE-----
      MIIC6jCCAdKgAwIBAgIBADANBgkqhkiG9w0BAQsFADAVMRMwEQYDVQQDEwprdWJl
      cm5ldGVzMB4XDTIxMTIwNDE1MDE0N1oXDTMxMTIwMjE1MDY0N1owFTETMBEGA1UE
      AxMKa3ViZXJuZXRlczCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALtM
      gUk3xcD0XeFq4HZyV8X2+mSLXlYC1UXu9VqnEbzFi5s0WJc6mvJo+4bddg0aoKyv
      x+GcW9dfyMkDnaBnByS99DJWSxR49etGli5Nc6zNASHJHX8Fz+r5CB6LZVQ3KNZZ
      GC9yPSGdsqARXPNP8+NmzCoquWzRiMzyQRUIwtFwrUjzFtYsHlbVwGAnT6Voowi3
      MdaMs8Yw4JyqRmvc+V7CLgIxT7H636GoQBAiB/nF4/Q6F0/it0m0ZpYZ6vpOXxd6
      a10zfq9kB+7XpdffkovtJCZmdofyecjzfbezo5XVC94Ni4RONMQHujOP7aCs0TTl
      3ZpsrGSMln4tqF4kTZkCAwEAAaNFMEMwDgYDVR0PAQH/BAQDAgKkMBIGA1UdEwEB
      /wQIMAYBAf8CAQAwHQYDVR0OBBYEFJAX5tkbi5W+eS6wpUL/yc0HF0ZSMA0GCSqG
      SIb3DQEBCwUAA4IBAQCXq1JXShGXy1teKchf/ceBhjjU71rfgMIS4Z6SMZ3StzWA
      OtJTABkP+Y7OkJZLf7xvVQvsKKGTGy6PcZN+7EB1xR/7QlpeIrvW8UGyO1rYOkPH
      QX36EIvAcnuzKL3IgiJNk0aBlt1mvUJ2feHGokIlllCMoh3ED6gT2NTo+vnNnFlO
      JiscjVRKS8GM4J5aS2STn664v1NIxM2bbEkWInO+f85086raDg9DR2RGhHaIhfmM
      Xgg9o1Xlo2bTMoXKoYMwOM6w17d1K6a8ltftuYNNVDeNrWSTeg2LJ5SjuuWXvMWY
      c/i7/OBAd8QgX++BJAQKVKK/J8QolorzzMT18s5H
      -----END CERTIFICATE-----
      
-   path: /etc/kubernetes/pki/ca.key
    owner: root:root
    permissions: '0600'
    content: |
      -----BEGIN RSA PRIVATE KEY-----
      MIIEowIBAAKCAQEAu0yBSTfFwPRd4WrgdnJXxfb6ZIteVgLVRe71WqcRvMWLmzRY
      lzqa8mj7ht12DRqgrK/H4Zxb11/IyQOdoGcHJL30MlZLFHj160aWLk1zrM0BIckd
      fwXP6vkIHotlVDco1lkYL3I9IZ2yoBFc80/z42bMKiq5bNGIzPJBFQjC0XCtSPMW
      1iweVtXAYCdPpWijCLcx1oyzxjDgnKpGa9z5XsIuAjFPsfrfoahAECIH+cXj9DoX
      T+K3SbRmlhnq+k5fF3prXTN+r2QH7tel19+Si+0kJmZ2h/J5yPN9t7OjldUL3g2L
      hE40xAe6M4/toKzRNOXdmmysZIyWfi2oXiRNmQIDAQABAoIBAQC1L48p6yAMRtjC
      hYdaTcaHJSKYPRInFlqGamFDLrdD673fiEXjFbhqpBAeKQJYLtgb9Xfg0kcuE+TC
      QBMt5jzM2EzwnPXIejM7RG9nn1k1YqOjsVAtXswBvKKUGbkOPMXuhQWWcGaerFTt
      754Baei+pOUALZBuqkwyJm+7D1yXCUc52UiLuwCmxbuJxC4zWgjcIbOVqReLzixV
      I7f7g4AnMpMw75VOzqczW1Q2KeT/wAknDY7tVHe1RfTvu6gEWsX5u2fQxhTDnIjp
      cmDedAf7EgoC1cE2kSfTFW0q90xJJObMk9OzfdtmuenZsvOW3rOEAEF09ycBiOyW
      zcjWCbrhAoGBAOPn42x3xA2ClGEjZr0VCprhr8bY82GbNjp49p907yMOrHNvOyLS
      BK8ktaaVr61LCjNH3qKb7jYNr3ONkzoAFfH7nZGxfN95zCcWEGM8MLTNHXTCqKDA
      pKBCEkWnDGQLMLhTffguLosnizfhlYMGs3PYVsWUIo2lj9nOZsy0eZLdAoGBANJj
      Kr4cRCRuLoNtf/kXFvAyzz4fhfdRV0/iAvxnPlsaZJbKnCbKxT/AaYDyEUSzgoyN
      m/NazpV/gvMflrT7qa6krAp9PqOgzfX/cNy8DENSBRZMUGnj8rPqiDiKOhzWXYx+
      vC+gD5iJTPaqBOTQ1Ts0mZpVWcuwpxdIrXHHjcPtAoGAWm3kW2GaNRIe9fwqA9SZ
      hKMQMAJdb9k6RzFACj1Htc1Yt+TmvgY/PY9/VD4ImuYvgfF+cV8VwfTkLSF7zYPD
      MWT5PJoERlf5nXivv/BeEx9gFLg4WLCXoc8VmPWTgQ6/oiPe097fMO/b2ax0uqyp
      /8lThMome7W5wl6Xg5oIszECgYBECK+ExM1AXqUJ+Tn+Cfpv+G5OL5F51cL/YR4I
      Ezb17QYEQUbXwJCiug0kFqOA7O/VleGNg5r0e0SUbG2m3w8TG8tKpQ/BiDmySEVu
      DB2HE5nziQAkDgOpLLmaVxDNzIB5823VlNQWRqgtx/NHL0UVHUBiySD9noWaIPV9
      qsNsTQKBgDV/MBwwx6cwmMkjhVSkaW0PpZjJWk259ArOrhgK5WfJT2GQ9n/xE9xE
      CnsGDBuUSOodx3jaF41SeH6vNMAUPcis6TmVwmuZ1wP1w5wKHWG2oqyvhb+rpix3
      rOpaEdseHPwIiR/aj3fU/BzxsXFwS4q7c1kEJdNVII8p6frsyT0t
      -----END RSA PRIVATE KEY-----
      
-   path: /etc/kubernetes/pki/etcd/ca.crt
    owner: root:root
    permissions: '0640'
    content: |
      -----BEGIN CERTIFICATE-----
      MIIC6jCCAdKgAwIBAgIBADANBgkqhkiG9w0BAQsFADAVMRMwEQYDVQQDEwprdWJl
      cm5ldGVzMB4XDTIxMTIwNDE1MDE0OFoXDTMxMTIwMjE1MDY0OFowFTETMBEGA1UE
      AxMKa3ViZXJuZXRlczCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAOAo
      1PvPCBUC51qet24fWqt07ggdzIhs4SZvMymgmLtbigSZpNfp7pQoqs10n2/4/LJi
      Vvyg0uZQor1NprEPDOqWhyH3afl/5+JjzNLCxt0Ny8C2nO3iWPikZhRqbsg7lMQY
      PyIIrKs0Z/nKo5ks8qT4nQjPQUfYKPdptyoP/G0SteNIGDdVdoe8xBb/sSncWHnM
      x7kAYikIQmq5fKprBfLXhxO9am4M2LQEl/VC+aA3GHTjZZG48uCsfU+ixZ1VlV17
      Ps5NWrnLE9uKbj38Yg70XVIWH0wuy+leLhdy69VMOaYyGpoWJg3Dtbjjs2Ras/TP
      kIiPxrijm2cuQ4KUW38CAwEAAaNFMEMwDgYDVR0PAQH/BAQDAgKkMBIGA1UdEwEB
      /wQIMAYBAf8CAQAwHQYDVR0OBBYEFKsaxLWnksWmZA3TKWCuqi4u6gIOMA0GCSqG
      SIb3DQEBCwUAA4IBAQChmUDJdjdLxF1mLBTRWCLhi73UcPo3SwNb5+Ej8KDLRBu5
      8YegBCLzQpqz+jaBe96QlvGib8eJ6Vfn2trCvO8KGucg1ZDdmYtmSg7GUP8x2yOR
      Ffjs21PVkChOCg7MXgapiNFUVktK0rcYBHq84zKD14M2ZU0a+oK+zHnHTm8TvFCh
      LYKcswX9w7N0YuyoVCdim0k3tTAz8dwHkHw1S+3i/oqu+v4PInVMbqNfvEfaeXIx
      ADr1beY13OGcT6TaFBs0xXsxbshwpZ2eva82/qbM6h0QL6fQDMWu9zO4sFaIY6fB
      E0sB8Lian7kf4eBoYO9NrNXH2frmSvNx1gsrAy6y
      -----END CERTIFICATE-----
      
-   path: /etc/kubernetes/pki/etcd/ca.key
    owner: root:root
    permissions: '0600'
    content: |
      -----BEGIN RSA PRIVATE KEY-----
      MIIEowIBAAKCAQEA4CjU+88IFQLnWp63bh9aq3TuCB3MiGzhJm8zKaCYu1uKBJmk
      1+nulCiqzXSfb/j8smJW/KDS5lCivU2msQ8M6paHIfdp+X/n4mPM0sLG3Q3LwLac
      7eJY+KRmFGpuyDuUxBg/IgisqzRn+cqjmSzypPidCM9BR9go92m3Kg/8bRK140gY
      N1V2h7zEFv+xKdxYeczHuQBiKQhCarl8qmsF8teHE71qbgzYtASX9UL5oDcYdONl
      kbjy4Kx9T6LFnVWVXXs+zk1aucsT24puPfxiDvRdUhYfTC7L6V4uF3Lr1Uw5pjIa
      mhYmDcO1uOOzZFqz9M+QiI/GuKObZy5DgpRbfwIDAQABAoIBAHWv+mJaR/wAEkdZ
      nSSMAaaTNYW9X20g/PSY3Vu1nXqAjO3tXMafY0sWLta/rBW1u7ZMOy9XoGKbY1XQ
      Nvwu0rE3ZqtGorUDmlMZ4qek65OTcq4zMiES/XNNnOqLFq652Vk7Aap0s3MPiKd0
      5H+/QYWroYbGiZeWvatoLWpACl+Yv3cersIuLrXUXWCz1olafAPxtMquHuGBPOJS
      QT2Xp/ENJW1JqEOoWmslpc4mEu4vbxDZozfHI7Sfca8nHx8WeAeJR0nJ1uFmELsu
      NccGCFj0yDUvnPMqWnnoU5qoAlwBHUcaeOqCnno0aD4YDRplQX8gPaA6FArIoKeS
      ZThnYEECgYEA5g/vKTfS6uRySpwwDO3lIKVdrWig3uPr7lNILLVttaWMTRJ0FmqJ
      UMvmtA1APV3GAXMNvJPUGUgf93g7/9cNB2VB+vZ4clpESFU5jDcB/wViHJfimNBS
      PZbaFc1KEUSVtkKh3vV/6FGt6ybbrv8OCOjQcic3roFKn5QPNMZrppUCgYEA+W6I
      U/+2FYSsmdJMSMVzovq5FPvkHTG0tt65uRKBWYW90N9s/972mXB6nfDZt6UM77ie
      VbYYElnZjj6LoQD9NR6a42VsgLkRpEqBU293+x8FLedwR8mZ6PDwrgW+NtpBTohZ
      afkFXA328PFhwEkRp2Iri6hLn3iqvXH11VX8mMMCgYAH/V2s7MdiaPSfKrVwfYKL
      k7KhJxUPKJM0/6duBg79U/Z/ZripXqHOMIaekic8+li6DCjZ97hR+HNDwOU0iV9m
      dlnIQW8FaaUdbfhFqlNja+hwXcX80J9KjEaeozaDSwJ4BfBhMd1zUALeO8c9WJZA
      MPWsQThp0wuoZxfwGUP70QKBgHgOM5/6nHGPAmSnTABayWXQt/TZqNpEam76lPn3
      ZjronIxEffpKHveLo/kRTDmQP8HCYrNuifeLN6O3hw1fpIBE0thQoQD0EwG4uram
      GGHOdHe7xddHucTc83tPWFaehoB+MEtJiMLeFdWy2RHsGYsvPTZjMsL3GXdFusWM
      NaBxAoGBAOHp7a31W9yx+0DIrO4Mg3yZCRk8+1Fz6x3glhXARa/kdDXJe7x0zk+q
      q0LthsY08dls+z/zZtP90hrOKxOJauTEWbd9YLKJu+2Em8IELKCm8U7IJLz7hdw0
      2AbEY8O28GBgdR3wdvGNJT4QrIWJJRaxmHuHIZ0kaVYahOp9Hhe9
      -----END RSA PRIVATE KEY-----
      
-   path: /etc/kubernetes/pki/front-proxy-ca.crt
    owner: root:root
    permissions: '0640'
    content: |
      -----BEGIN CERTIFICATE-----
      MIIC6jCCAdKgAwIBAgIBADANBgkqhkiG9w0BAQsFADAVMRMwEQYDVQQDEwprdWJl
      cm5ldGVzMB4XDTIxMTIwNDE1MDE0N1oXDTMxMTIwMjE1MDY0N1owFTETMBEGA1UE
      AxMKa3ViZXJuZXRlczCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALWq
      slEXKxWRWIxwpgfk3PZP17aDvO97DZnasGp1pjF+nKOio5JV5GeoflZzVflRV2wY
      hUz8dUtcFx0k5c9wV4ieGc3qxWj0ULkStlXkfOORDntfMRyCYvGWFtOY/UFeS5i/
      blV8mhmzFGaRvME+vssfkbc/GgXYmmCApNw3cO/xfDA/c0SiDc82ikpzTcdl6dXT
      CA1Rcpy4SMprE29LYa5/ET/czB1SYrc1R6JjTwfFFfGxmVsJQEM27Ikb4KpUbl7m
      617thxwJQET3vs1yxmssJnjz33OSurgTuxSSFo9cNceuBuHHzBJUoj1BtCi0FVQZ
      SEDpwj3d53SKB5sliUECAwEAAaNFMEMwDgYDVR0PAQH/BAQDAgKkMBIGA1UdEwEB
      /wQIMAYBAf8CAQAwHQYDVR0OBBYEFOs3tu4sXYyc5OgPenEi5xRyYgCnMA0GCSqG
      SIb3DQEBCwUAA4IBAQBTw4YPVhzFppKEcTUTOgFlVytseCz0s3gwG4SRNDXcyLKP
      9/BQYVKY4jqsaLxlQNs5HzwRSSMLU/GxxqnlUTkJcGnujkNWNQw58VFN/IcOt55B
      lb+gv2thCtK3DMRxMZy0dH2LCKL0pgbUfItSzw8kt55dPUkHnrzafosf8X54cWKb
      FXDXJostyTtKbciodLNU6DJ6WRO1TiSBDf8opZXN3rK7jpWvM1VH4HaMj/NeQctY
      qJnlcbgL0IZoFuMc7reoyTmk1rJk28OhRKszLkE32oNaH34JvJQWyJCSGJHUIZ4P
      LsO3ptGxStE1ryqanEFxDb7A6Hh9P0+aPsDQXP9w
      -----END CERTIFICATE-----
      
-   path: /etc/kubernetes/pki/front-proxy-ca.key
    owner: root:root
    permissions: '0600'
    content: |
      -----BEGIN RSA PRIVATE KEY-----
      MIIEpQIBAAKCAQEAtaqyURcrFZFYjHCmB+Tc9k/XtoO873sNmdqwanWmMX6co6Kj
      klXkZ6h+VnNV+VFXbBiFTPx1S1wXHSTlz3BXiJ4ZzerFaPRQuRK2VeR845EOe18x
      HIJi8ZYW05j9QV5LmL9uVXyaGbMUZpG8wT6+yx+Rtz8aBdiaYICk3Ddw7/F8MD9z
      RKINzzaKSnNNx2Xp1dMIDVFynLhIymsTb0thrn8RP9zMHVJitzVHomNPB8UV8bGZ
      WwlAQzbsiRvgqlRuXubrXu2HHAlARPe+zXLGaywmePPfc5K6uBO7FJIWj1w1x64G
      4cfMElSiPUG0KLQVVBlIQOnCPd3ndIoHmyWJQQIDAQABAoIBACsyy/Q8biJSzZuX
      reNyqJhppAHikargt/s95XVrRHnAgb7njb3ebtG3X1NvWaJPlVo++nO0FLA21cg4
      Xe1V6XqzHa+5g/fRIODhcjo6evgiJi9wE12UI7MO3Z6zYoWIxrEr1DC/0GUMEG3T
      ee753KSwfRX2C1oYh50q+gjjphdoDlBhQz+nnDtEUD6YyKeJItqRd7gt7ZkiWFsH
      6eLo69ZKaM3yiEw9zi93TaeuZxigNQv/6clCCSpbmm8ZVGAa+uESu/0s7bUvNE2C
      MBDWJvY8ClaR1OFy/UZv8uaHsSt1xJLSWdKtS1Ex8wtkErdc05ztkXZ87+bno9Y/
      QzvtSAECgYEA4oF/gMpCFk1OHNHG1pzbbO5XniEYKMMAtqSDnlwWDsaeojc3/DX/
      SQNitqI0lydJ0WRTQ6NZdRIY2IN7E2VFIwIumR1gKsQ8hprTS4qrl/hj6Czbk2To
      +H0h2jH4N0QapRmPCeeQQn39u+Xb4zu9b0rEzSGe/HCtDGVN+OeuB4kCgYEAzVKA
      i26hvimWPALGoTbdX2fvz75l4BNHUQInggn2hvW0SuNl5Gy+bZdYqKwj9lDatvJ6
      XO0cTeEjQZRI9VQNRiub+8kHIwKxPBWI5H4TibYEr4kin1JYTR1KTM2px1MG8+Pj
      YUwaqJ5fPaVXp3kPs0fdcZf1gnP3YapBa9K8TfkCgYEAti8kyAlm+Js7TfDpNuu5
      jjdy3+yMixS1+TH/75rv3vig1acGb+VarXZ8qptzI3TlafeBBXFY3dIti9DNaL9W
      yZ7SrjMzi5KFgFr9wtAJztVqPm/+OOK8hEnZta/lj0ZHDC6vn27S2LiQItbycyY0
      61Q/USNOxos2lTbSbXajwskCgYEAykCDyVWgU/8JM3IUkYfHBw0OSJioJ9M1xBGY
      M1t3EbiE8eQQYbPQ3YlhVz3Cawd4exBeAp267OhiX14fhDJYpQ+eJqb+tbkYNzSL
      VXDv9A5tjTBL/58Qxl2c7A0HOgaKacLJH/XkqMbg0IvHzXvOQG8BLr1epTNwsy8Q
      JJNA1JkCgYEAh2oTnDfp860filAkoLZTUhBkTouOFuvw53Wkuw7HKs0I3QkjPFT6
      QxPdBwjOUVPdTNMY/FScxC+twRBfGq9JJ8Ua/DLFz9S7rU+ekOtj2jXNEeZOSlvx
      YKeRhJUzzQMaVZTcbmgIte4DU9muroftsZZ9PJQsq5KoqYgZHD1i7cI=
      -----END RSA PRIVATE KEY-----
      
-   path: /etc/kubernetes/pki/sa.pub
    owner: root:root
    permissions: '0640'
    content: |
      -----BEGIN PUBLIC KEY-----
      MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA7Unos35cvi+gED50wDvv
      JHwqONBS82PbM0OxcWrEyayWOjNp9NLJIFAYuDn7ix3qQIbNvSeEicre2o3GDLJh
      2Bi+Rhv1Ea/0IF0nnKIve6he51hfQAhnW9h7mOv0o0utGL96JQn45B2kCRUCbgS1
      1L1YwqyXh0lBDFv5NzgGJQPXbbzL9Pxitpyiu1nj2dxM2955Qgn0b+cULaV3BR7v
      HNuQi+dO9I3SKyRXqWArX3F7oISFSjAdJXUKwWOxzj0rLxOVbIsOul+bW8yHd/Z6
      KJb8a2udasWIJ7vM4ezE3M47Ypx+14gUubBd/mC5pY14t1xlbJs53rIQFsiP19Az
      ywIDAQAB
      -----END PUBLIC KEY-----
      
-   path: /etc/kubernetes/pki/sa.key
    owner: root:root
    permissions: '0600'
    content: |
      -----BEGIN RSA PRIVATE KEY-----
      MIIEowIBAAKCAQEA7Unos35cvi+gED50wDvvJHwqONBS82PbM0OxcWrEyayWOjNp
      9NLJIFAYuDn7ix3qQIbNvSeEicre2o3GDLJh2Bi+Rhv1Ea/0IF0nnKIve6he51hf
      QAhnW9h7mOv0o0utGL96JQn45B2kCRUCbgS11L1YwqyXh0lBDFv5NzgGJQPXbbzL
      9Pxitpyiu1nj2dxM2955Qgn0b+cULaV3BR7vHNuQi+dO9I3SKyRXqWArX3F7oISF
      SjAdJXUKwWOxzj0rLxOVbIsOul+bW8yHd/Z6KJb8a2udasWIJ7vM4ezE3M47Ypx+
      14gUubBd/mC5pY14t1xlbJs53rIQFsiP19AzywIDAQABAoIBACr+4GZduCpR8Nvg
      pUEL2xouUWw3+z/U0Swp0OYvJXcxiYsEM+fDpePv/3qqLvUXN5H3myHyHibllpnd
      ZIx6ahZA7YFAoZhR3JdcqcfM73Oln4Sl06SDoU7YHBUqdAp+tN+uGlDJzMpwwH9Q
      yj7rJZNlt8aWhWJjGKFHrRGfWiWsgjEqMrq1HIs0W8Yn3v+/3wmJBjJrNEeaoNhV
      Uedp6ULFAdhtb5ecI9Tt5UR4sTqmm8YMkPjYyYbM/IHu6xm3QN6dfjHkym6zkIkU
      FxnIztgJl3gzGt8wTJefDhfFlShz8p62AV4i1bsZkE6jocp5wNE0FmqLwUgWnuF7
      ORxdtgkCgYEA+lbLeVsVynDYP5B9/qu1sGMlwtgIrI7nBC7HB8UIMq3haepT6UVn
      EHzU//IQue9ILFOhPCNtmH4zd4FDgG/vrkP9j/9fmrqrGfjtTgU1gQ8nYC3ueV3c
      8MugZMGaahOBJDws8uANqVQTvZ9MlC4AN3bF/Xa2Sk+/fnY9Io6rMpUCgYEA8qeQ
      8IlN+yba7/j1yG4jMCfr1QRJROLxQ5sjdTTA2z8iaGUZjTLlv/91sJznG/8uEJ4L
      xWFyE98O+OBKrsdNtmT89BXnWD5EI/vKSa8eLQOcKNhkN+YfSdFYs7ZDi75Y8hl3
      SMoNJTo1nTKxOr4Iy//1gZRGxOuVe4cwuFZtlN8CgYAc4bCd8qVD8trwEnKG1Dak
      //tWTGhLyDzc3ay2t8OnXSo5dwBxVEF8xHoqgTnuya1w98ENWCUHx9+WNQKdqcxk
      NZHmcBcOmeStnWt7adxvZFktnn7535ti6Is7tJ5lCJUIoiypZLIOzBVu9hb2rYv2
      2iwjfvOvBR5Zr7iD6SPVNQKBgC26Aggx964CbnOWWMrCZoMmorxrqFsA4TI6Q/5M
      SKOITDWcB6qiEsWRoF3901dlSQr8nX8+k77G5A1mRuyUxkI+2aQtlID+ity1EDO+
      elNFQOI5lPkrtm20s6B6ElR9NEm7Hs1qtftz8rKC4P8O3J2EyID4rjVhp7O1kCrM
      rq3FAoGBAMk02oYIKzVJZ6wg/yfc8rgh1E0N8SCX2go2TnhTgzmJiw0iIiH02WN1
      3hgTCH9Lg7kYZHdUikG+eTfmnuVbdu42cwy7LO1wa8k76z+SaAyH/EOG7pzpMEzC
      1TW7DfWgeYrnx3PV2Chqk9ml+dgyKNaTPPCBAXu7tYzAjes0iW1Q
      -----END RSA PRIVATE KEY-----
      
-   path: /run/kubeadm/kubeadm.yaml
    owner: root:root
    permissions: '0640'
    content: |
      ---
      apiServer:
        certSANs:
        - localhost
        - 127.0.0.1
      apiVersion: kubeadm.k8s.io/v1beta3
      clusterName: sample
      controlPlaneEndpoint: 172.18.0.3:6443
      controllerManager:
        extraArgs:
          enable-hostpath-provisioner: "true"
      dns: {}
      etcd: {}
      kind: ClusterConfiguration
      kubernetesVersion: v1.22.0
      networking:
        dnsDomain: cluster.local
        podSubnet: 100.96.0.0/11
        serviceSubnet: 10.128.0.0/12
      scheduler: {}
      
      ---
      apiVersion: kubeadm.k8s.io/v1beta3
      kind: InitConfiguration
      localAPIEndpoint: {}
      nodeRegistration:
        criSocket: /var/run/containerd/containerd.sock
        kubeletExtraArgs:
          cgroup-driver: cgroupfs
          eviction-hard: nodefs.available<0%,nodefs.inodesFree<0%,imagefs.available<0%
        taints: null
      
-   path: /run/cluster-api/placeholder
    owner: root:root
    permissions: '0640'
    content: "This placeholder file is used to create the /run/cluster-api sub directory in a way that is compatible with both Linux and Windows (mkdir -p /run/cluster-api does not work with Windows)"
runcmd:
  - 'kubeadm init --config /run/kubeadm/kubeadm.yaml  && echo success > /run/cluster-api/bootstrap-success.complete'

`

const playbook = `
- become: true
  connection: local
  hosts: localhost
  name: cloud-init
  tasks:
  - file:
      dest: /etc/kubernetes/pki
      state: directory
  - file:
      dest: /etc/kubernetes/pki/etcd
      state: directory
  - file:
      dest: /run/cluster-api
      state: directory
  - file:
      dest: /run/kubeadm
      state: directory
  - copy:
      content: |
        -----BEGIN CERTIFICATE-----
        MIIC6jCCAdKgAwIBAgIBADANBgkqhkiG9w0BAQsFADAVMRMwEQYDVQQDEwprdWJl
        cm5ldGVzMB4XDTIxMTIwNDE1MDE0N1oXDTMxMTIwMjE1MDY0N1owFTETMBEGA1UE
        AxMKa3ViZXJuZXRlczCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALtM
        gUk3xcD0XeFq4HZyV8X2+mSLXlYC1UXu9VqnEbzFi5s0WJc6mvJo+4bddg0aoKyv
        x+GcW9dfyMkDnaBnByS99DJWSxR49etGli5Nc6zNASHJHX8Fz+r5CB6LZVQ3KNZZ
        GC9yPSGdsqARXPNP8+NmzCoquWzRiMzyQRUIwtFwrUjzFtYsHlbVwGAnT6Voowi3
        MdaMs8Yw4JyqRmvc+V7CLgIxT7H636GoQBAiB/nF4/Q6F0/it0m0ZpYZ6vpOXxd6
        a10zfq9kB+7XpdffkovtJCZmdofyecjzfbezo5XVC94Ni4RONMQHujOP7aCs0TTl
        3ZpsrGSMln4tqF4kTZkCAwEAAaNFMEMwDgYDVR0PAQH/BAQDAgKkMBIGA1UdEwEB
        /wQIMAYBAf8CAQAwHQYDVR0OBBYEFJAX5tkbi5W+eS6wpUL/yc0HF0ZSMA0GCSqG
        SIb3DQEBCwUAA4IBAQCXq1JXShGXy1teKchf/ceBhjjU71rfgMIS4Z6SMZ3StzWA
        OtJTABkP+Y7OkJZLf7xvVQvsKKGTGy6PcZN+7EB1xR/7QlpeIrvW8UGyO1rYOkPH
        QX36EIvAcnuzKL3IgiJNk0aBlt1mvUJ2feHGokIlllCMoh3ED6gT2NTo+vnNnFlO
        JiscjVRKS8GM4J5aS2STn664v1NIxM2bbEkWInO+f85086raDg9DR2RGhHaIhfmM
        Xgg9o1Xlo2bTMoXKoYMwOM6w17d1K6a8ltftuYNNVDeNrWSTeg2LJ5SjuuWXvMWY
        c/i7/OBAd8QgX++BJAQKVKK/J8QolorzzMT18s5H
        -----END CERTIFICATE-----
      dest: /etc/kubernetes/pki/ca.crt
      group: root
      mode: "0640"
      owner: root
  - copy:
      content: |
        -----BEGIN RSA PRIVATE KEY-----
        MIIEowIBAAKCAQEAu0yBSTfFwPRd4WrgdnJXxfb6ZIteVgLVRe71WqcRvMWLmzRY
        lzqa8mj7ht12DRqgrK/H4Zxb11/IyQOdoGcHJL30MlZLFHj160aWLk1zrM0BIckd
        fwXP6vkIHotlVDco1lkYL3I9IZ2yoBFc80/z42bMKiq5bNGIzPJBFQjC0XCtSPMW
        1iweVtXAYCdPpWijCLcx1oyzxjDgnKpGa9z5XsIuAjFPsfrfoahAECIH+cXj9DoX
        T+K3SbRmlhnq+k5fF3prXTN+r2QH7tel19+Si+0kJmZ2h/J5yPN9t7OjldUL3g2L
        hE40xAe6M4/toKzRNOXdmmysZIyWfi2oXiRNmQIDAQABAoIBAQC1L48p6yAMRtjC
        hYdaTcaHJSKYPRInFlqGamFDLrdD673fiEXjFbhqpBAeKQJYLtgb9Xfg0kcuE+TC
        QBMt5jzM2EzwnPXIejM7RG9nn1k1YqOjsVAtXswBvKKUGbkOPMXuhQWWcGaerFTt
        754Baei+pOUALZBuqkwyJm+7D1yXCUc52UiLuwCmxbuJxC4zWgjcIbOVqReLzixV
        I7f7g4AnMpMw75VOzqczW1Q2KeT/wAknDY7tVHe1RfTvu6gEWsX5u2fQxhTDnIjp
        cmDedAf7EgoC1cE2kSfTFW0q90xJJObMk9OzfdtmuenZsvOW3rOEAEF09ycBiOyW
        zcjWCbrhAoGBAOPn42x3xA2ClGEjZr0VCprhr8bY82GbNjp49p907yMOrHNvOyLS
        BK8ktaaVr61LCjNH3qKb7jYNr3ONkzoAFfH7nZGxfN95zCcWEGM8MLTNHXTCqKDA
        pKBCEkWnDGQLMLhTffguLosnizfhlYMGs3PYVsWUIo2lj9nOZsy0eZLdAoGBANJj
        Kr4cRCRuLoNtf/kXFvAyzz4fhfdRV0/iAvxnPlsaZJbKnCbKxT/AaYDyEUSzgoyN
        m/NazpV/gvMflrT7qa6krAp9PqOgzfX/cNy8DENSBRZMUGnj8rPqiDiKOhzWXYx+
        vC+gD5iJTPaqBOTQ1Ts0mZpVWcuwpxdIrXHHjcPtAoGAWm3kW2GaNRIe9fwqA9SZ
        hKMQMAJdb9k6RzFACj1Htc1Yt+TmvgY/PY9/VD4ImuYvgfF+cV8VwfTkLSF7zYPD
        MWT5PJoERlf5nXivv/BeEx9gFLg4WLCXoc8VmPWTgQ6/oiPe097fMO/b2ax0uqyp
        /8lThMome7W5wl6Xg5oIszECgYBECK+ExM1AXqUJ+Tn+Cfpv+G5OL5F51cL/YR4I
        Ezb17QYEQUbXwJCiug0kFqOA7O/VleGNg5r0e0SUbG2m3w8TG8tKpQ/BiDmySEVu
        DB2HE5nziQAkDgOpLLmaVxDNzIB5823VlNQWRqgtx/NHL0UVHUBiySD9noWaIPV9
        qsNsTQKBgDV/MBwwx6cwmMkjhVSkaW0PpZjJWk259ArOrhgK5WfJT2GQ9n/xE9xE
        CnsGDBuUSOodx3jaF41SeH6vNMAUPcis6TmVwmuZ1wP1w5wKHWG2oqyvhb+rpix3
        rOpaEdseHPwIiR/aj3fU/BzxsXFwS4q7c1kEJdNVII8p6frsyT0t
        -----END RSA PRIVATE KEY-----
      dest: /etc/kubernetes/pki/ca.key
      group: root
      mode: "0600"
      owner: root
  - copy:
      content: |
        -----BEGIN CERTIFICATE-----
        MIIC6jCCAdKgAwIBAgIBADANBgkqhkiG9w0BAQsFADAVMRMwEQYDVQQDEwprdWJl
        cm5ldGVzMB4XDTIxMTIwNDE1MDE0OFoXDTMxMTIwMjE1MDY0OFowFTETMBEGA1UE
        AxMKa3ViZXJuZXRlczCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAOAo
        1PvPCBUC51qet24fWqt07ggdzIhs4SZvMymgmLtbigSZpNfp7pQoqs10n2/4/LJi
        Vvyg0uZQor1NprEPDOqWhyH3afl/5+JjzNLCxt0Ny8C2nO3iWPikZhRqbsg7lMQY
        PyIIrKs0Z/nKo5ks8qT4nQjPQUfYKPdptyoP/G0SteNIGDdVdoe8xBb/sSncWHnM
        x7kAYikIQmq5fKprBfLXhxO9am4M2LQEl/VC+aA3GHTjZZG48uCsfU+ixZ1VlV17
        Ps5NWrnLE9uKbj38Yg70XVIWH0wuy+leLhdy69VMOaYyGpoWJg3Dtbjjs2Ras/TP
        kIiPxrijm2cuQ4KUW38CAwEAAaNFMEMwDgYDVR0PAQH/BAQDAgKkMBIGA1UdEwEB
        /wQIMAYBAf8CAQAwHQYDVR0OBBYEFKsaxLWnksWmZA3TKWCuqi4u6gIOMA0GCSqG
        SIb3DQEBCwUAA4IBAQChmUDJdjdLxF1mLBTRWCLhi73UcPo3SwNb5+Ej8KDLRBu5
        8YegBCLzQpqz+jaBe96QlvGib8eJ6Vfn2trCvO8KGucg1ZDdmYtmSg7GUP8x2yOR
        Ffjs21PVkChOCg7MXgapiNFUVktK0rcYBHq84zKD14M2ZU0a+oK+zHnHTm8TvFCh
        LYKcswX9w7N0YuyoVCdim0k3tTAz8dwHkHw1S+3i/oqu+v4PInVMbqNfvEfaeXIx
        ADr1beY13OGcT6TaFBs0xXsxbshwpZ2eva82/qbM6h0QL6fQDMWu9zO4sFaIY6fB
        E0sB8Lian7kf4eBoYO9NrNXH2frmSvNx1gsrAy6y
        -----END CERTIFICATE-----
      dest: /etc/kubernetes/pki/etcd/ca.crt
      group: root
      mode: "0640"
      owner: root
  - copy:
      content: |
        -----BEGIN RSA PRIVATE KEY-----
        MIIEowIBAAKCAQEA4CjU+88IFQLnWp63bh9aq3TuCB3MiGzhJm8zKaCYu1uKBJmk
        1+nulCiqzXSfb/j8smJW/KDS5lCivU2msQ8M6paHIfdp+X/n4mPM0sLG3Q3LwLac
        7eJY+KRmFGpuyDuUxBg/IgisqzRn+cqjmSzypPidCM9BR9go92m3Kg/8bRK140gY
        N1V2h7zEFv+xKdxYeczHuQBiKQhCarl8qmsF8teHE71qbgzYtASX9UL5oDcYdONl
        kbjy4Kx9T6LFnVWVXXs+zk1aucsT24puPfxiDvRdUhYfTC7L6V4uF3Lr1Uw5pjIa
        mhYmDcO1uOOzZFqz9M+QiI/GuKObZy5DgpRbfwIDAQABAoIBAHWv+mJaR/wAEkdZ
        nSSMAaaTNYW9X20g/PSY3Vu1nXqAjO3tXMafY0sWLta/rBW1u7ZMOy9XoGKbY1XQ
        Nvwu0rE3ZqtGorUDmlMZ4qek65OTcq4zMiES/XNNnOqLFq652Vk7Aap0s3MPiKd0
        5H+/QYWroYbGiZeWvatoLWpACl+Yv3cersIuLrXUXWCz1olafAPxtMquHuGBPOJS
        QT2Xp/ENJW1JqEOoWmslpc4mEu4vbxDZozfHI7Sfca8nHx8WeAeJR0nJ1uFmELsu
        NccGCFj0yDUvnPMqWnnoU5qoAlwBHUcaeOqCnno0aD4YDRplQX8gPaA6FArIoKeS
        ZThnYEECgYEA5g/vKTfS6uRySpwwDO3lIKVdrWig3uPr7lNILLVttaWMTRJ0FmqJ
        UMvmtA1APV3GAXMNvJPUGUgf93g7/9cNB2VB+vZ4clpESFU5jDcB/wViHJfimNBS
        PZbaFc1KEUSVtkKh3vV/6FGt6ybbrv8OCOjQcic3roFKn5QPNMZrppUCgYEA+W6I
        U/+2FYSsmdJMSMVzovq5FPvkHTG0tt65uRKBWYW90N9s/972mXB6nfDZt6UM77ie
        VbYYElnZjj6LoQD9NR6a42VsgLkRpEqBU293+x8FLedwR8mZ6PDwrgW+NtpBTohZ
        afkFXA328PFhwEkRp2Iri6hLn3iqvXH11VX8mMMCgYAH/V2s7MdiaPSfKrVwfYKL
        k7KhJxUPKJM0/6duBg79U/Z/ZripXqHOMIaekic8+li6DCjZ97hR+HNDwOU0iV9m
        dlnIQW8FaaUdbfhFqlNja+hwXcX80J9KjEaeozaDSwJ4BfBhMd1zUALeO8c9WJZA
        MPWsQThp0wuoZxfwGUP70QKBgHgOM5/6nHGPAmSnTABayWXQt/TZqNpEam76lPn3
        ZjronIxEffpKHveLo/kRTDmQP8HCYrNuifeLN6O3hw1fpIBE0thQoQD0EwG4uram
        GGHOdHe7xddHucTc83tPWFaehoB+MEtJiMLeFdWy2RHsGYsvPTZjMsL3GXdFusWM
        NaBxAoGBAOHp7a31W9yx+0DIrO4Mg3yZCRk8+1Fz6x3glhXARa/kdDXJe7x0zk+q
        q0LthsY08dls+z/zZtP90hrOKxOJauTEWbd9YLKJu+2Em8IELKCm8U7IJLz7hdw0
        2AbEY8O28GBgdR3wdvGNJT4QrIWJJRaxmHuHIZ0kaVYahOp9Hhe9
        -----END RSA PRIVATE KEY-----
      dest: /etc/kubernetes/pki/etcd/ca.key
      group: root
      mode: "0600"
      owner: root
  - copy:
      content: |
        -----BEGIN CERTIFICATE-----
        MIIC6jCCAdKgAwIBAgIBADANBgkqhkiG9w0BAQsFADAVMRMwEQYDVQQDEwprdWJl
        cm5ldGVzMB4XDTIxMTIwNDE1MDE0N1oXDTMxMTIwMjE1MDY0N1owFTETMBEGA1UE
        AxMKa3ViZXJuZXRlczCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALWq
        slEXKxWRWIxwpgfk3PZP17aDvO97DZnasGp1pjF+nKOio5JV5GeoflZzVflRV2wY
        hUz8dUtcFx0k5c9wV4ieGc3qxWj0ULkStlXkfOORDntfMRyCYvGWFtOY/UFeS5i/
        blV8mhmzFGaRvME+vssfkbc/GgXYmmCApNw3cO/xfDA/c0SiDc82ikpzTcdl6dXT
        CA1Rcpy4SMprE29LYa5/ET/czB1SYrc1R6JjTwfFFfGxmVsJQEM27Ikb4KpUbl7m
        617thxwJQET3vs1yxmssJnjz33OSurgTuxSSFo9cNceuBuHHzBJUoj1BtCi0FVQZ
        SEDpwj3d53SKB5sliUECAwEAAaNFMEMwDgYDVR0PAQH/BAQDAgKkMBIGA1UdEwEB
        /wQIMAYBAf8CAQAwHQYDVR0OBBYEFOs3tu4sXYyc5OgPenEi5xRyYgCnMA0GCSqG
        SIb3DQEBCwUAA4IBAQBTw4YPVhzFppKEcTUTOgFlVytseCz0s3gwG4SRNDXcyLKP
        9/BQYVKY4jqsaLxlQNs5HzwRSSMLU/GxxqnlUTkJcGnujkNWNQw58VFN/IcOt55B
        lb+gv2thCtK3DMRxMZy0dH2LCKL0pgbUfItSzw8kt55dPUkHnrzafosf8X54cWKb
        FXDXJostyTtKbciodLNU6DJ6WRO1TiSBDf8opZXN3rK7jpWvM1VH4HaMj/NeQctY
        qJnlcbgL0IZoFuMc7reoyTmk1rJk28OhRKszLkE32oNaH34JvJQWyJCSGJHUIZ4P
        LsO3ptGxStE1ryqanEFxDb7A6Hh9P0+aPsDQXP9w
        -----END CERTIFICATE-----
      dest: /etc/kubernetes/pki/front-proxy-ca.crt
      group: root
      mode: "0640"
      owner: root
  - copy:
      content: |
        -----BEGIN RSA PRIVATE KEY-----
        MIIEpQIBAAKCAQEAtaqyURcrFZFYjHCmB+Tc9k/XtoO873sNmdqwanWmMX6co6Kj
        klXkZ6h+VnNV+VFXbBiFTPx1S1wXHSTlz3BXiJ4ZzerFaPRQuRK2VeR845EOe18x
        HIJi8ZYW05j9QV5LmL9uVXyaGbMUZpG8wT6+yx+Rtz8aBdiaYICk3Ddw7/F8MD9z
        RKINzzaKSnNNx2Xp1dMIDVFynLhIymsTb0thrn8RP9zMHVJitzVHomNPB8UV8bGZ
        WwlAQzbsiRvgqlRuXubrXu2HHAlARPe+zXLGaywmePPfc5K6uBO7FJIWj1w1x64G
        4cfMElSiPUG0KLQVVBlIQOnCPd3ndIoHmyWJQQIDAQABAoIBACsyy/Q8biJSzZuX
        reNyqJhppAHikargt/s95XVrRHnAgb7njb3ebtG3X1NvWaJPlVo++nO0FLA21cg4
        Xe1V6XqzHa+5g/fRIODhcjo6evgiJi9wE12UI7MO3Z6zYoWIxrEr1DC/0GUMEG3T
        ee753KSwfRX2C1oYh50q+gjjphdoDlBhQz+nnDtEUD6YyKeJItqRd7gt7ZkiWFsH
        6eLo69ZKaM3yiEw9zi93TaeuZxigNQv/6clCCSpbmm8ZVGAa+uESu/0s7bUvNE2C
        MBDWJvY8ClaR1OFy/UZv8uaHsSt1xJLSWdKtS1Ex8wtkErdc05ztkXZ87+bno9Y/
        QzvtSAECgYEA4oF/gMpCFk1OHNHG1pzbbO5XniEYKMMAtqSDnlwWDsaeojc3/DX/
        SQNitqI0lydJ0WRTQ6NZdRIY2IN7E2VFIwIumR1gKsQ8hprTS4qrl/hj6Czbk2To
        +H0h2jH4N0QapRmPCeeQQn39u+Xb4zu9b0rEzSGe/HCtDGVN+OeuB4kCgYEAzVKA
        i26hvimWPALGoTbdX2fvz75l4BNHUQInggn2hvW0SuNl5Gy+bZdYqKwj9lDatvJ6
        XO0cTeEjQZRI9VQNRiub+8kHIwKxPBWI5H4TibYEr4kin1JYTR1KTM2px1MG8+Pj
        YUwaqJ5fPaVXp3kPs0fdcZf1gnP3YapBa9K8TfkCgYEAti8kyAlm+Js7TfDpNuu5
        jjdy3+yMixS1+TH/75rv3vig1acGb+VarXZ8qptzI3TlafeBBXFY3dIti9DNaL9W
        yZ7SrjMzi5KFgFr9wtAJztVqPm/+OOK8hEnZta/lj0ZHDC6vn27S2LiQItbycyY0
        61Q/USNOxos2lTbSbXajwskCgYEAykCDyVWgU/8JM3IUkYfHBw0OSJioJ9M1xBGY
        M1t3EbiE8eQQYbPQ3YlhVz3Cawd4exBeAp267OhiX14fhDJYpQ+eJqb+tbkYNzSL
        VXDv9A5tjTBL/58Qxl2c7A0HOgaKacLJH/XkqMbg0IvHzXvOQG8BLr1epTNwsy8Q
        JJNA1JkCgYEAh2oTnDfp860filAkoLZTUhBkTouOFuvw53Wkuw7HKs0I3QkjPFT6
        QxPdBwjOUVPdTNMY/FScxC+twRBfGq9JJ8Ua/DLFz9S7rU+ekOtj2jXNEeZOSlvx
        YKeRhJUzzQMaVZTcbmgIte4DU9muroftsZZ9PJQsq5KoqYgZHD1i7cI=
        -----END RSA PRIVATE KEY-----
      dest: /etc/kubernetes/pki/front-proxy-ca.key
      group: root
      mode: "0600"
      owner: root
  - copy:
      content: |
        -----BEGIN PUBLIC KEY-----
        MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA7Unos35cvi+gED50wDvv
        JHwqONBS82PbM0OxcWrEyayWOjNp9NLJIFAYuDn7ix3qQIbNvSeEicre2o3GDLJh
        2Bi+Rhv1Ea/0IF0nnKIve6he51hfQAhnW9h7mOv0o0utGL96JQn45B2kCRUCbgS1
        1L1YwqyXh0lBDFv5NzgGJQPXbbzL9Pxitpyiu1nj2dxM2955Qgn0b+cULaV3BR7v
        HNuQi+dO9I3SKyRXqWArX3F7oISFSjAdJXUKwWOxzj0rLxOVbIsOul+bW8yHd/Z6
        KJb8a2udasWIJ7vM4ezE3M47Ypx+14gUubBd/mC5pY14t1xlbJs53rIQFsiP19Az
        ywIDAQAB
        -----END PUBLIC KEY-----
      dest: /etc/kubernetes/pki/sa.pub
      group: root
      mode: "0640"
      owner: root
  - copy:
      content: |
        -----BEGIN RSA PRIVATE KEY-----
        MIIEowIBAAKCAQEA7Unos35cvi+gED50wDvvJHwqONBS82PbM0OxcWrEyayWOjNp
        9NLJIFAYuDn7ix3qQIbNvSeEicre2o3GDLJh2Bi+Rhv1Ea/0IF0nnKIve6he51hf
        QAhnW9h7mOv0o0utGL96JQn45B2kCRUCbgS11L1YwqyXh0lBDFv5NzgGJQPXbbzL
        9Pxitpyiu1nj2dxM2955Qgn0b+cULaV3BR7vHNuQi+dO9I3SKyRXqWArX3F7oISF
        SjAdJXUKwWOxzj0rLxOVbIsOul+bW8yHd/Z6KJb8a2udasWIJ7vM4ezE3M47Ypx+
        14gUubBd/mC5pY14t1xlbJs53rIQFsiP19AzywIDAQABAoIBACr+4GZduCpR8Nvg
        pUEL2xouUWw3+z/U0Swp0OYvJXcxiYsEM+fDpePv/3qqLvUXN5H3myHyHibllpnd
        ZIx6ahZA7YFAoZhR3JdcqcfM73Oln4Sl06SDoU7YHBUqdAp+tN+uGlDJzMpwwH9Q
        yj7rJZNlt8aWhWJjGKFHrRGfWiWsgjEqMrq1HIs0W8Yn3v+/3wmJBjJrNEeaoNhV
        Uedp6ULFAdhtb5ecI9Tt5UR4sTqmm8YMkPjYyYbM/IHu6xm3QN6dfjHkym6zkIkU
        FxnIztgJl3gzGt8wTJefDhfFlShz8p62AV4i1bsZkE6jocp5wNE0FmqLwUgWnuF7
        ORxdtgkCgYEA+lbLeVsVynDYP5B9/qu1sGMlwtgIrI7nBC7HB8UIMq3haepT6UVn
        EHzU//IQue9ILFOhPCNtmH4zd4FDgG/vrkP9j/9fmrqrGfjtTgU1gQ8nYC3ueV3c
        8MugZMGaahOBJDws8uANqVQTvZ9MlC4AN3bF/Xa2Sk+/fnY9Io6rMpUCgYEA8qeQ
        8IlN+yba7/j1yG4jMCfr1QRJROLxQ5sjdTTA2z8iaGUZjTLlv/91sJznG/8uEJ4L
        xWFyE98O+OBKrsdNtmT89BXnWD5EI/vKSa8eLQOcKNhkN+YfSdFYs7ZDi75Y8hl3
        SMoNJTo1nTKxOr4Iy//1gZRGxOuVe4cwuFZtlN8CgYAc4bCd8qVD8trwEnKG1Dak
        //tWTGhLyDzc3ay2t8OnXSo5dwBxVEF8xHoqgTnuya1w98ENWCUHx9+WNQKdqcxk
        NZHmcBcOmeStnWt7adxvZFktnn7535ti6Is7tJ5lCJUIoiypZLIOzBVu9hb2rYv2
        2iwjfvOvBR5Zr7iD6SPVNQKBgC26Aggx964CbnOWWMrCZoMmorxrqFsA4TI6Q/5M
        SKOITDWcB6qiEsWRoF3901dlSQr8nX8+k77G5A1mRuyUxkI+2aQtlID+ity1EDO+
        elNFQOI5lPkrtm20s6B6ElR9NEm7Hs1qtftz8rKC4P8O3J2EyID4rjVhp7O1kCrM
        rq3FAoGBAMk02oYIKzVJZ6wg/yfc8rgh1E0N8SCX2go2TnhTgzmJiw0iIiH02WN1
        3hgTCH9Lg7kYZHdUikG+eTfmnuVbdu42cwy7LO1wa8k76z+SaAyH/EOG7pzpMEzC
        1TW7DfWgeYrnx3PV2Chqk9ml+dgyKNaTPPCBAXu7tYzAjes0iW1Q
        -----END RSA PRIVATE KEY-----
      dest: /etc/kubernetes/pki/sa.key
      group: root
      mode: "0600"
      owner: root
  - copy:
      content: |
        ---
        apiServer:
          certSANs:
          - localhost
          - 127.0.0.1
        apiVersion: kubeadm.k8s.io/v1beta3
        clusterName: sample
        controlPlaneEndpoint: 172.18.0.3:6443
        controllerManager:
          extraArgs:
            enable-hostpath-provisioner: "true"
        dns: {}
        etcd: {}
        kind: ClusterConfiguration
        kubernetesVersion: v1.22.0
        networking:
          dnsDomain: cluster.local
          podSubnet: 100.96.0.0/11
          serviceSubnet: 10.128.0.0/12
        scheduler: {}

        ---
        apiVersion: kubeadm.k8s.io/v1beta3
        kind: InitConfiguration
        localAPIEndpoint: {}
        nodeRegistration:
          criSocket: /var/run/containerd/containerd.sock
          kubeletExtraArgs:
            cgroup-driver: cgroupfs
            eviction-hard: nodefs.available<0%,nodefs.inodesFree<0%,imagefs.available<0%
          taints: null
      dest: /run/kubeadm/kubeadm.yaml
      group: root
      mode: "0640"
      owner: root
  - copy:
      content: This placeholder file is used to create the /run/cluster-api sub directory
        in a way that is compatible with both Linux and Windows (mkdir -p /run/cluster-api
        does not work with Windows)
      dest: /run/cluster-api/placeholder
      group: root
      mode: "0640"
      owner: root
  - shell:
      cmd: kubeadm init --config /run/kubeadm/kubeadm.yaml  && echo success > /run/cluster-api/bootstrap-success.complete
  vars_files:
  - variables.yaml
`

func TestRealUseCase(t *testing.T) {
	g := NewWithT(t)
	adapter := NewAnsibleAdapter(bootstrapv1.KubeadmConfigSpec{})
	resultPlaybook, err := adapter.userDataToPlaybook([]byte(cloudData))
	g.Expect(err).NotTo(HaveOccurred())
	expectedPlaybook := strings.TrimSpace(playbook)
	gotPlaybook := strings.TrimSpace(string(resultPlaybook))
	if !cmp.Equal(expectedPlaybook, gotPlaybook) {
		t.Errorf("playbook is not as expected,\ngot: %s\ndiff: %s", gotPlaybook, cmp.Diff(expectedPlaybook, gotPlaybook))
	}
}
