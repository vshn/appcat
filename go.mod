module github.com/vshn/appcat/v4

go 1.22.3

replace github.com/deepmap/oapi-codegen => github.com/sljeff/oapi-codegen v1.5.1-0.20211207091501-bc20f55d338a

require (
	dario.cat/mergo v1.0.0
	github.com/alecthomas/assert/v2 v2.1.0
	github.com/appuio/appuio-cloud-reporting v0.15.0
	github.com/appuio/openshift-upgrade-controller v0.5.1
	github.com/blang/semver/v4 v4.0.0
	github.com/cert-manager/cert-manager v1.13.2
	github.com/crossplane-contrib/provider-kubernetes v0.13.0
	github.com/crossplane/crossplane v1.16.0
	github.com/crossplane/crossplane-runtime v1.16.0
	github.com/crossplane/crossplane-tools v0.0.0-20230925130601-628280f8bf79
	github.com/crossplane/function-sdk-go v0.3.0-rc.0.0.20240521211021-11b59ad46986
	github.com/deepmap/oapi-codegen v0.0.0-00010101000000-000000000000
	github.com/go-logr/logr v1.4.1
	github.com/go-logr/zapr v1.3.0
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.4
	github.com/hashicorp/go-version v1.6.0
	github.com/iancoleman/strcase v0.2.0
	github.com/jackc/pgx/v5 v5.3.1
	github.com/k8up-io/k8up/v2 v2.7.1
	github.com/minio/minio-go/v7 v7.0.63
	github.com/ory/dockertest v3.3.5+incompatible
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.63.0
	github.com/prometheus/client_golang v1.18.0
	github.com/prometheus/common v0.45.0
	github.com/sethvargo/go-password v0.2.0
	github.com/spf13/cobra v1.8.0
	github.com/spf13/viper v1.15.0
	github.com/stretchr/testify v1.9.0
	github.com/thediveo/enumflag/v2 v2.0.2
	github.com/vektra/mockery/v2 v2.36.1
	github.com/vshn/provider-minio v0.0.0-20230906065457-d7b41bd371e8
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.63.2
	k8s.io/api v0.30.0
	k8s.io/apimachinery v0.30.0
	k8s.io/client-go v0.30.0
	k8s.io/code-generator v0.30.0
	k8s.io/utils v0.0.0-20240102154912-e7106e64919e
	sigs.k8s.io/controller-runtime v0.18.2
	sigs.k8s.io/controller-tools v0.14.0
	sigs.k8s.io/kind v0.20.0
	sigs.k8s.io/yaml v1.4.0
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/alecthomas/repr v0.1.0 // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/alessio/shellescape v1.4.1 // indirect
	github.com/chigopher/pathlib v0.15.0 // indirect
	github.com/dave/jennifer v1.6.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/getkin/kin-openapi v0.80.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-json-experiment/json v0.0.0-20231102232822-2e55bd4e08b0 // indirect
	github.com/gobuffalo/flect v1.0.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/safetext v0.0.0-20220905092116-b49f7bc46da2 // indirect
	github.com/hexops/gotextdiff v1.0.3 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/jinzhu/copier v0.3.5 // indirect
	github.com/labstack/echo/v4 v4.2.1 // indirect
	github.com/labstack/gommon v0.3.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/openshift/api v0.0.0-20230213134911-7ba313770556 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rs/zerolog v1.29.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.1 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240415180920-8c6c420018be // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6 // indirect
	k8s.io/gengo/v2 v2.0.0-20240228010128-51d4e06bde70 // indirect
	sigs.k8s.io/gateway-api v0.8.0 // indirect
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/containerd/continuity v0.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch v5.7.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.9.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-sql-driver/mysql v1.8.1
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gotestyourself/gotestyourself v2.2.0+incompatible // indirect
	github.com/hashicorp/hcl v1.0.1-vault-5 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.7 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/opencontainers/runc v1.1.7 // indirect
	github.com/pelletier/go-toml/v2 v2.0.6 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/redis/go-redis/v9 v9.0.5
	github.com/rs/xid v1.5.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/exp v0.0.0-20240416160154-fe59bbe5cc7f // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/oauth2 v0.18.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/term v0.19.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.20.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/protobuf v1.33.0
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools v2.2.0+incompatible // indirect
	helm.sh/helm/v3 v3.15.1
	k8s.io/apiextensions-apiserver v0.30.0 // indirect
	k8s.io/klog/v2 v2.120.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
)
