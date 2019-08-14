package operator

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/appscode/go/log"
	api "github.com/appscode/kubed/apis/kubed/v1alpha1"
	es "github.com/appscode/kubed/pkg/elasticsearch"
	"github.com/appscode/kubed/pkg/eventer"
	influx "github.com/appscode/kubed/pkg/influxdb"
	rbin "github.com/appscode/kubed/pkg/recyclebin"
	indexers "github.com/appscode/kubed/pkg/registry/resource"
	"github.com/appscode/kubed/pkg/syncer"
	searchlight_api "github.com/appscode/searchlight/apis/monitoring/v1alpha1"
	srch_cs "github.com/appscode/searchlight/client/clientset/versioned"
	searchlightinformers "github.com/appscode/searchlight/client/informers/externalversions"
	voyager_api "github.com/appscode/voyager/apis/voyager/v1beta1"
	vcs "github.com/appscode/voyager/client/clientset/versioned"
	voyagerinformers "github.com/appscode/voyager/client/informers/externalversions"
	shell "github.com/codeskyblue/go-sh"
	promapi "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	prominformers "github.com/coreos/prometheus-operator/pkg/client/informers/externalversions"
	pcm "github.com/coreos/prometheus-operator/pkg/client/versioned"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"gomodules.xyz/envconfig"
	certificates "k8s.io/api/certificates/v1beta1"
	core "k8s.io/api/core/v1"
	storage_v1 "k8s.io/api/storage/v1"
	_ "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/dynamiclister"
	"k8s.io/client-go/informers"
	core_informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	_ "kmodules.xyz/client-go/apiextensions/v1beta1"
	"kmodules.xyz/client-go/discovery"
	"kmodules.xyz/client-go/tools/backup"
	"kmodules.xyz/client-go/tools/fsnotify"
	"kmodules.xyz/client-go/tools/queue"
	storage "kmodules.xyz/objectstore-api/osm"
	kubedb_api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	kcs "kubedb.dev/apimachinery/client/clientset/versioned"
	kubedbinformers "kubedb.dev/apimachinery/client/informers/externalversions"
	stash_api "stash.appscode.dev/stash/apis/stash/v1alpha1"
	scs "stash.appscode.dev/stash/client/clientset/versioned"
	stashinformers "stash.appscode.dev/stash/client/informers/externalversions"
)

type Operator struct {
	Config

	ClientConfig *rest.Config

	notifierCred   envconfig.LoaderFunc
	recorder       record.EventRecorder
	trashCan       *rbin.RecycleBin
	eventProcessor *eventer.EventForwarder
	configSyncer   *syncer.ConfigSyncer

	cron *cron.Cron

	KubeClient        kubernetes.Interface
	DynamicClient dynamic.Interface

	Mapper meta.RESTMapper
	Factory       dynamicinformer.DynamicSharedInformerFactory
	Listers       map[schema.GroupVersionResource]dynamiclister.Lister
	syncedFns     []cache.InformerSynced

	Indexer *indexers.ResourceIndexer

	watcher *fsnotify.Watcher

	clusterConfig api.ClusterConfig
	lock          sync.RWMutex
}

func (op *Operator) Configure() error {
	log.Infoln("configuring kubed ...")

	op.lock.Lock()
	defer op.lock.Unlock()

	var err error

	cfg, err := api.LoadConfig(op.ConfigPath)
	if err != nil {
		return err
	}
	err = cfg.Validate()
	if err != nil {
		return err
	}
	op.clusterConfig = *cfg

	if op.clusterConfig.RecycleBin != nil && op.clusterConfig.RecycleBin.Path == "" {
		op.clusterConfig.RecycleBin.Path = filepath.Join(op.ScratchDir, "trashcan")
	}

	op.notifierCred, err = op.getLoader()
	if err != nil {
		return err
	}

	err = op.trashCan.Configure(op.clusterConfig.ClusterName, op.clusterConfig.RecycleBin)
	if err != nil {
		return err
	}

	err = op.eventProcessor.Configure(op.clusterConfig.ClusterName, op.clusterConfig.EventForwarder, op.notifierCred)
	if err != nil {
		return err
	}

	err = op.configSyncer.Configure(op.clusterConfig.ClusterName, op.clusterConfig.KubeConfigFile, op.clusterConfig.EnableConfigSyncer)
	if err != nil {
		return err
	}

	for _, j := range op.clusterConfig.Janitors {
		if j.Kind == api.JanitorInfluxDB {
			janitor := influx.Janitor{Spec: *j.InfluxDB, TTL: j.TTL.Duration}
			err = janitor.Cleanup()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (op *Operator) setupInformers() {
	resources := []schema.GroupVersionResource{
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "replicationcontrollers"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
		schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobd"},

		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "Service"},
		schema.GroupVersionResource{Group: "extensions", Version: "v1beta1", Resource: "Ingress"},
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "NetworkPolicy"},

		schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
		schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
		schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},

		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"},
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "limitranges"},
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"},
	}

	for _, gvr := range resources {
		i := op.Factory.ForResource(gvr).Informer()
		if gvk, err := op.Mapper.KindFor(gvr); err == nil {
			op.addEventHandlers(i, gvk)
		}
		op.syncedFns = append(op.syncedFns, i.HasSynced)
	}
}

func (op *Operator) setupConfigInformers() {
	configMapInformer := op.kubeInformerFactory.InformerFor(&core.ConfigMap{}, func(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
		return core_informers.NewFilteredConfigMapInformer(
			client,
			op.clusterConfig.ConfigSourceNamespace,
			resyncPeriod,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			func(options *metav1.ListOptions) {},
		)
	})
	op.addEventHandlers(configMapInformer, core.SchemeGroupVersion.WithKind("ConfigMap"))
	configMapInformer.AddEventHandler(op.configSyncer.ConfigMapHandler())

	secretInformer := op.kubeInformerFactory.InformerFor(&core.Secret{}, func(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
		return core_informers.NewFilteredSecretInformer(
			client,
			op.clusterConfig.ConfigSourceNamespace,
			resyncPeriod,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			func(options *metav1.ListOptions) {},
		)
	})
	op.addEventHandlers(secretInformer, core.SchemeGroupVersion.WithKind("Secret"))
	secretInformer.AddEventHandler(op.configSyncer.SecretHandler())

	nsInformer := op.kubeInformerFactory.Core().V1().Namespaces().Informer()
	op.addEventHandlers(nsInformer, core.SchemeGroupVersion.WithKind("Namespace"))
	nsInformer.AddEventHandler(op.configSyncer.NamespaceHandler())
}

func (op *Operator) setupEventInformers() {
	eventInformer := op.kubeInformerFactory.InformerFor(&core.Event{}, func(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
		return core_informers.NewFilteredEventInformer(
			client,
			core.NamespaceAll,
			resyncPeriod,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			func(options *metav1.ListOptions) {
				options.FieldSelector = fields.OneTermEqualSelector("type", core.EventTypeWarning).String()
			},
		)
	})
	eventInformer.AddEventHandler(op.eventProcessor)
}

func (op *Operator) setupCertificateInformers() {
	op.addEventHandlers(csrInformer, certificates.SchemeGroupVersion.WithKind("CertificateSigningRequest"))
}

func (op *Operator) setupStorageInformers() {
	op.addEventHandlers(pvInformer, core.SchemeGroupVersion.WithKind("PersistentVolume"))
	op.addEventHandlers(pvcInformer, core.SchemeGroupVersion.WithKind("PersistentVolumeClaim"))
	op.addEventHandlers(storageClassInformer, storage_v1.SchemeGroupVersion.WithKind("StorageClass"))
}

func (op *Operator) setupVoyagerInformers() {
	if discovery.IsPreferredAPIResource(op.KubeClient.Discovery(), voyager_api.SchemeGroupVersion.String(), voyager_api.ResourceKindIngress) {
		op.addEventHandlers(voyagerIngressInformer, voyager_api.SchemeGroupVersion.WithKind(voyager_api.ResourceKindIngress))
		op.addEventHandlers(voyagerCertificateInformer, voyager_api.SchemeGroupVersion.WithKind(voyager_api.ResourceKindCertificate))
	}
}

func (op *Operator) setupStashInformers() {
	if discovery.IsPreferredAPIResource(op.KubeClient.Discovery(), stash_api.SchemeGroupVersion.String(), stash_api.ResourceKindRestic) {
		op.addEventHandlers(resticsInformer, stash_api.SchemeGroupVersion.WithKind(stash_api.ResourceKindRestic))
		op.addEventHandlers(recoveryInformer, stash_api.SchemeGroupVersion.WithKind(stash_api.ResourceKindRecovery))
	}
}

func (op *Operator) setupSearchlightInformers() {
	if discovery.IsPreferredAPIResource(op.KubeClient.Discovery(), searchlight_api.SchemeGroupVersion.String(), searchlight_api.ResourceKindClusterAlert) {
		op.addEventHandlers(clusterAlertInformer, searchlight_api.SchemeGroupVersion.WithKind(searchlight_api.ResourceKindClusterAlert))
		op.addEventHandlers(nodeAlertInformer, searchlight_api.SchemeGroupVersion.WithKind(searchlight_api.ResourceKindNodeAlert))
		op.addEventHandlers(podAlertInformer, searchlight_api.SchemeGroupVersion.WithKind(searchlight_api.ResourceKindPodAlert))
	}
}

func (op *Operator) setupKubeDBInformers() {
	if discovery.IsPreferredAPIResource(op.KubeClient.Discovery(), kubedb_api.SchemeGroupVersion.String(), kubedb_api.ResourceKindPostgres) {
		op.addEventHandlers(pgInformer, kubedb_api.SchemeGroupVersion.WithKind(kubedb_api.ResourceKindPostgres))
		op.addEventHandlers(esInformer, kubedb_api.SchemeGroupVersion.WithKind(kubedb_api.ResourceKindElasticsearch))
		op.addEventHandlers(myInformer, kubedb_api.SchemeGroupVersion.WithKind(kubedb_api.ResourceKindMySQL))
		op.addEventHandlers(mgInformer, kubedb_api.SchemeGroupVersion.WithKind(kubedb_api.ResourceKindMongoDB))
		op.addEventHandlers(rdInformer, kubedb_api.SchemeGroupVersion.WithKind(kubedb_api.ResourceKindRedis))
		op.addEventHandlers(mcInformer, kubedb_api.SchemeGroupVersion.WithKind(kubedb_api.ResourceKindMemcached))
		op.addEventHandlers(dbSnapshotInformer, kubedb_api.SchemeGroupVersion.WithKind(kubedb_api.ResourceKindSnapshot))
		op.addEventHandlers(dormantDatabaseInformer, kubedb_api.SchemeGroupVersion.WithKind(kubedb_api.ResourceKindDormantDatabase))
	}
}

func (op *Operator) setupPrometheusInformers() {
	if discovery.IsPreferredAPIResource(op.KubeClient.Discovery(), promapi.SchemeGroupVersion.String(), promapi.PrometheusesKind) {
		op.addEventHandlers(promInf, promapi.SchemeGroupVersion.WithKind(promapi.PrometheusesKind))
		op.addEventHandlers(ruleInf, promapi.SchemeGroupVersion.WithKind(promapi.PrometheusRuleKind))
		op.addEventHandlers(smonInf, promapi.SchemeGroupVersion.WithKind(promapi.ServiceMonitorsKind))
		op.addEventHandlers(amgrInf, promapi.SchemeGroupVersion.WithKind(promapi.AlertmanagersKind))
	}
}

func (op *Operator) addEventHandlers(informer cache.SharedIndexInformer, gvk schema.GroupVersionKind) {
	informer.AddEventHandler(queue.NewVersionedHandler(op.trashCan, gvk))
	informer.AddEventHandler(queue.NewVersionedHandler(op.eventProcessor, gvk))
	informer.AddEventHandler(queue.NewVersionedHandler(op.Indexer, gvk))
}

func (op *Operator) getLoader() (envconfig.LoaderFunc, error) {
	if op.clusterConfig.NotifierSecretName == "" {
		return func(key string) (string, bool) {
			return "", false
		}, nil
	}
	cfg, err := op.KubeClient.CoreV1().
		Secrets(op.OperatorNamespace).
		Get(op.clusterConfig.NotifierSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return func(key string) (value string, found bool) {
		var bytes []byte
		bytes, found = cfg.Data[key]
		value = string(bytes)
		return
	}, nil
}

func (op *Operator) RunWatchers(stopCh <-chan struct{}) {
	op.Factory.Start(stopCh)
	if ok := cache.WaitForCacheSync(stopCh, op.syncedFns...); !ok {
		log.Errorf("failed to wait for caches to sync")
	}
}

func (op *Operator) RunElasticsearchCleaner() error {
	for _, j := range op.clusterConfig.Janitors {
		if j.Kind == api.JanitorElasticsearch {
			var authInfo *api.JanitorAuthInfo

			if j.Elasticsearch.SecretName != "" {
				secret, err := op.KubeClient.CoreV1().Secrets(op.OperatorNamespace).
					Get(j.Elasticsearch.SecretName, metav1.GetOptions{})
				if err != nil && !kerr.IsNotFound(err) {
					return err
				}
				if secret != nil {
					authInfo = api.LoadJanitorAuthInfo(secret.Data)
				}
			}

			janitor := es.Janitor{Spec: *j.Elasticsearch, AuthInfo: authInfo, TTL: j.TTL.Duration}
			err := janitor.Cleanup()
			if err != nil {
				return err
			}
			op.cron.AddFunc("@every 1h", func() {
				err := janitor.Cleanup()
				if err != nil {
					log.Errorln(err)
				}
			})
		}
	}
	return nil
}

func (op *Operator) RunTrashCanCleaner() error {
	if op.trashCan == nil {
		return nil
	}

	schedule := "@every 1h"
	if op.Test {
		schedule = "@every 1m"
	}

	_, err := op.cron.AddFunc(schedule, func() {
		err := op.trashCan.Cleanup()
		if err != nil {
			log.Errorln(err)
		}
	})
	return err
}

func (op *Operator) RunSnapshotter() error {
	if op.clusterConfig.Snapshotter == nil {
		return nil
	}

	osmconfigPath := filepath.Join(op.ScratchDir, "osm", "config.yaml")
	err := storage.WriteOSMConfig(op.KubeClient, op.OperatorNamespace, op.clusterConfig.Snapshotter.Backend, osmconfigPath)
	if err != nil {
		return err
	}

	container, err := api.Container(op.clusterConfig.Snapshotter.Backend)
	if err != nil {
		return err
	}

	// test credentials
	sh := shell.NewSession()
	sh.SetDir(op.ScratchDir)
	sh.ShowCMD = true
	snapshotter := func() error {
		mgr := backup.NewBackupManager(op.clusterConfig.ClusterName, op.ClientConfig, op.clusterConfig.Snapshotter.Sanitize)
		snapshotFile, err := mgr.BackupToTar(filepath.Join(op.ScratchDir, "snapshot"))
		if err != nil {
			return err
		}
		defer func() {
			if err := os.Remove(snapshotFile); err != nil {
				log.Errorln(err)
			}
		}()
		dest, err := op.clusterConfig.Snapshotter.Location(filepath.Base(snapshotFile))
		if err != nil {
			return err
		}
		return sh.Command("osm", "push", "--osmconfig", osmconfigPath, "-c", container, snapshotFile, dest).Run()
	}
	// start taking first backup
	go func() {
		err := snapshotter()
		if err != nil {
			log.Errorln(err)
		}
	}()

	if !op.Test { // don't run cronjob for test. it cause problem for consecutive tests.
		_, err := op.cron.AddFunc(op.clusterConfig.Snapshotter.Schedule, func() {
			err := snapshotter()
			if err != nil {
				log.Errorln(err)
			}
		})
		return err
	}
	return nil
}

func (op *Operator) Run(stopCh <-chan struct{}) {
	if err := op.RunElasticsearchCleaner(); err != nil {
		log.Fatalln(err.Error())
	}

	if err := op.RunTrashCanCleaner(); err != nil {
		log.Fatalln(err.Error())
	}

	if err := op.RunSnapshotter(); err != nil {
		log.Fatalln(err.Error())
	}

	op.RunWatchers(stopCh)
	go op.watcher.Run(stopCh)

	<-stopCh
	log.Infoln("Stopping kubed controller")
}
