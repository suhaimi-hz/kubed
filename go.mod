module github.com/appscode/kubed

go 1.12

require (
	github.com/RoaringBitmap/roaring v0.4.18 // indirect
	github.com/Smerity/govarint v0.0.0-20150407073650-7265e41f48f1 // indirect
	github.com/appscode/go v0.0.0-20190722173419-e454bf744023
	github.com/appscode/searchlight v0.0.0-20190604163604-8a6c4c21504d
	github.com/appscode/voyager v0.0.0-20190802225652-9dc50134054c
	github.com/blevesearch/bleve v0.7.0
	github.com/blevesearch/blevex v0.0.0-20180227211930-4b158bb555a3 // indirect
	github.com/blevesearch/go-porterstemmer v1.0.2 // indirect
	github.com/blevesearch/segment v0.0.0-20160915185041-762005e7a34f // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/codeskyblue/go-sh v0.0.0-20190412065543-76bd3d59ff27
	github.com/coreos/prometheus-operator v0.31.1
	github.com/couchbase/vellum v0.0.0-20190626091642-41f2deade2cf // indirect
	github.com/cznic/b v0.0.0-20181122101859-a26611c4d92d // indirect
	github.com/cznic/mathutil v0.0.0-20181122101859-297441e03548 // indirect
	github.com/cznic/strutil v0.0.0-20181122101858-275e90344537 // indirect
	github.com/edsrzf/mmap-go v1.0.0 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.19.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/influxdata/influxdb v1.5.3
	github.com/jmhodges/levigo v1.0.0 // indirect
	github.com/json-iterator/go v1.1.6
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/common v0.4.1
	github.com/remyoudompheng/bigfft v0.0.0-20190728182440-6a916e37a237 // indirect
	github.com/robfig/cron/v3 v3.0.0
	github.com/spf13/afero v1.2.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	github.com/steveyen/gtreap v0.0.0-20150807155958-0abe01ef9be2 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/tecbot/gorocksdb v0.0.0-20190705090504-162552197222 // indirect
	gomodules.xyz/cert v1.0.0
	gomodules.xyz/envconfig v1.3.1-0.20190308184047-426f31af0d45
	gomodules.xyz/notify v0.0.0-20190424183923-af47cb5a07a4
	gomodules.xyz/stow v0.2.0
	gopkg.in/olivere/elastic.v5 v5.0.61
	k8s.io/api v0.0.0-20190515023547-db5a9d1c40eb
	k8s.io/apiextensions-apiserver v0.0.0-20190516231611-bf6753f2aa24
	k8s.io/apimachinery v0.0.0-20190515023456-b74e4c97951f
	k8s.io/apiserver v0.0.0-20190516230822-f89599b3f645
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/kube-aggregator v0.0.0-20190515024249-81a6edcf70be
	k8s.io/kube-openapi v0.0.0-20190510232812-a01b7d5d6c22
	kmodules.xyz/client-go v0.0.0-20190802200916-043217632b6a
	kmodules.xyz/monitoring-agent-api v0.0.0-20190802203207-a87aa5b2e057
	kmodules.xyz/objectstore-api v0.0.0-20190802205146-9816ffafe0d7
	kmodules.xyz/webhook-runtime v0.0.0-20190802202019-9e77ee949266
	kubedb.dev/apimachinery v0.13.0-rc.0
	stash.appscode.dev/stash v0.9.0-rc.0
)

replace (
	contrib.go.opencensus.io/exporter/ocagent => contrib.go.opencensus.io/exporter/ocagent v0.3.0
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.3.0+incompatible
	github.com/census-instrumentation/opencensus-proto => github.com/census-instrumentation/opencensus-proto v0.1.0
	github.com/golang/protobuf => github.com/golang/protobuf v1.2.0
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.4
	go.opencensus.io => go.opencensus.io v0.21.0
	k8s.io/api => k8s.io/api v0.0.0-20190313235455-40a48860b5ab
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190315093550-53c4693659ed
	k8s.io/apimachinery => github.com/kmodules/apimachinery v0.0.0-20190508045248-a52a97a7a2bf
	k8s.io/apiserver => github.com/kmodules/apiserver v0.0.0-20190508082252-8397d761d4b5
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190314001948-2899ed30580f
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190314002645-c892ea32361a
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190314000054-4a91899592f4
	k8s.io/klog => k8s.io/klog v0.3.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190314000639-da8327669ac5
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20190228160746-b3a7cee44a30
	k8s.io/metrics => k8s.io/metrics v0.0.0-20190314001731-1bd6a4002213
	k8s.io/utils => k8s.io/utils v0.0.0-20190221042446-c2654d5206da
)
