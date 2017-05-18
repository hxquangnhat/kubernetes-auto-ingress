package main

import (
    "os"
    "time"
    "flag"
    "reflect"

    "k8s.io/client-go/rest"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/cache"
    "k8s.io/client-go/tools/clientcmd"
    core "k8s.io/client-go/pkg/api/v1"
    extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"

    "k8s.io/apimachinery/pkg/fields"
    "k8s.io/apimachinery/pkg/util/intstr"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    log "github.com/Sirupsen/logrus"
)

var (
    //dns wildcard record for all applications created, should be like example.com
    wildcardRecord = os.Getenv("AUTO_INGRESS_SERVER_NAME")
    //namespace where auto-ingress will be activated
    namespace = os.Getenv("AUTO_INGRESS_NAMESPACE")
    //secret for ssl/tls of namespace where auto-ingress is running
    secret = os.Getenv("AUTO_INGRESS_SECRET")
    //read kubeconfig
    kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
)

func main() {
    flag.Parse()

    var err error
    var config *rest.Config

    //if kubeconfig is specified, use out-of-cluster
    if *kubeconfig != "" {
        config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
    } else {
        //get config when running inside Kubernetes
        config, err = rest.InClusterConfig()
    }

    if err != nil {
        log.Errorln(err.Error())
        return
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        log.Errorln(err.Error())
        return
    }

    //map to keep track of which services have been already auto-ingressed
	var svcIngPair map[string]extensions.Ingress
    svcIngPair = make(map[string]extensions.Ingress)

    //get current ingresses on cluster
    log.Info("Initializing mapping between ingresses and services...")
	err = createIngressServiceMap(clientset, svcIngPair)
    if err != nil {
        log.Errorln(err.Error())
        return
    }

    log.Info("Initialized map: ", reflect.ValueOf(svcIngPair).MapKeys())

    //create a watch to listen for create/update/delete event on service
    //new created service will be auto-ingressed if it specifies label "autoingress: true"
    //deleted service will be remove the associated ingress if it specifies label "autoingress: true"
    watchlist := cache.NewListWatchFromClient(clientset.Core().RESTClient(), "services", namespace,
        fields.Everything())
    _, controller := cache.NewInformer(
        watchlist,
        &core.Service{},
        time.Second * 0,
        cache.ResourceEventHandlerFuncs{
            AddFunc: func(obj interface{}) {
                svc := obj.(*core.Service)
                log.Info("Service added: ", svc.Name)
                lb := svc.Labels
                if _, found1 := svcIngPair[svc.Name]; !found1 {
                    if val, found2 := lb["auto-ingress/enabled"]; found2 {
                        if val == "enabled" {
                            newIng, err := createIngressForService(clientset, *svc)
                            if err != nil {
                                log.Errorln(err.Error())
                            } else {
                                log.Info("Created new ingress for service: ", svc.Name)
                                svcIngPair[svc.Name] = *newIng
                                log.Info("Updated map: ", reflect.ValueOf(svcIngPair).MapKeys())
                            }
                        }
                    }
                }
            },
            DeleteFunc: func(obj interface{}) {
                svc := obj.(*core.Service)
                log.Info("Service deleted: ", svc.Name)
				if ing, found := svcIngPair[svc.Name]; found {
					clientset.Ingresses(namespace).Delete(ing.Name, nil)
					log.Info("Deleted ingress for service: ", svc.Name)
                    delete(svcIngPair, svc.Name)
                    log.Info("Updated map: ", reflect.ValueOf(svcIngPair).MapKeys())
				}
            },
            UpdateFunc:func(oldObj, newObj interface{}) {
                newSvc := newObj.(*core.Service)
                log.Info("Service changed: ", newSvc.Name)
                lb := newSvc.Labels
                if ing, found1 := svcIngPair[newSvc.Name]; found1 {
                    if val, found2 := lb["auto-ingress/enabled"]; !found2 {
                        clientset.Ingresses(namespace).Delete(ing.Name, nil)
                        log.Info("Deleted ingress for service: ", newSvc.Name)
                        delete(svcIngPair, newSvc.Name)
                        log.Info("Updated map: ", reflect.ValueOf(svcIngPair).MapKeys())
                    } else {
                        if val == "disabled" {
                            clientset.Ingresses(namespace).Delete(ing.Name, nil)
                            log.Info("Deleted ingress for service: ", newSvc.Name)
                            delete(svcIngPair, newSvc.Name)
                            log.Info("Updated map: ", reflect.ValueOf(svcIngPair).MapKeys())
                        }
                    }
                } else {
                    if val, found3 := lb["auto-ingress/enabled"]; found3 {
                        if val == "enabled" {
                            newIng, err := createIngressForService(clientset, *newSvc)
                            if err != nil {
                                log.Errorln(err.Error())
                            } else {
                                log.Info("created new ingress for service: ", newSvc.Name)
                                svcIngPair[newSvc.Name] = *newIng
                                log.Info("Updated map: ", reflect.ValueOf(svcIngPair).MapKeys())
                            }
                        }
                    }
                }
            },
        },
    )
    stop := make(chan struct{})
    go controller.Run(stop)
    for{
        time.Sleep(time.Second)
    }
}

//create service map in the initial phase to check the current ingresses running on cluster
func createIngressServiceMap(clientset *kubernetes.Clientset, m map[string]extensions.Ingress) error {

	services, err := clientset.CoreV1().Services(namespace).List(metav1.ListOptions{})

    if err != nil {
        return err
    }

    ingresses, err:= clientset.ExtensionsV1beta1().Ingresses(namespace).List(metav1.ListOptions{})

    if err != nil {
        return err
    }

    for i:=0; i < len(ingresses.Items); i++ {
        rules := ingresses.Items[i].Spec.Rules
        for j:=0; j < len(rules); j++ {

            paths := rules[j].HTTP.Paths
            for k:=0; k < len(paths); k++ {
                svcName := paths[k].Backend.ServiceName
                if _, found := m[svcName]; !found {
                    m[svcName] = ingresses.Items[i]
                }
            }
        }
    }

	for i:=0; i < len(services.Items); i++ {
        if _, found1 := m[services.Items[i].GetName()]; !found1 {
            lb := services.Items[i].GetLabels()
            if val, found2 := lb["auto-ingress/enabled"]; found2 {
                if val == "enabled" {
                    newIng, err := createIngressForService(clientset, services.Items[i])
                    if err != nil {
                        return err
                    }
                    m[services.Items[i].GetName()] = *newIng
                }
            }
        }
    }

    return nil
}

//create an ingress for the associated service
func createIngressForService(clientset *kubernetes.Clientset, service core.Service) (*extensions.Ingress, error) {
	backend := createIngressBackend(service)

    ingressname := service.Name
    servername := ingressname + "." + wildcardRecord

    ingress := createIngress(ingressname, servername, backend)

    newIng, err := clientset.Ingresses(namespace).Create(ingress)

    return newIng, err
}

//create an ingress backend before putting it to the ingress
func createIngressBackend(service core.Service) extensions.IngressBackend {
    serviceName := service.GetName()
    if len(service.Spec.Ports) > 0 {
        var servicePort32 interface{}
        var servicePort int

        servicePort32 = service.Spec.Ports[0].Port
        servicePort32Tmp := servicePort32.(int32)

        servicePort = int(servicePort32Tmp)
        return extensions.IngressBackend {
            ServiceName: serviceName,
            ServicePort: intstr.FromInt(servicePort),
        }
    }

    return extensions.IngressBackend {}
}

//create ingress for associated service
func createIngress(ingressname string, servername string, backend extensions.IngressBackend) *extensions.Ingress {

    return &extensions.Ingress {
        ObjectMeta: metav1.ObjectMeta {
            Name: ingressname,
            Namespace: namespace,
        },
        Spec: extensions.IngressSpec {
            TLS: []extensions.IngressTLS{
                {
                    Hosts: []string{
                        servername,
                    },
                    SecretName: secret,

                },
            },
            Rules: []extensions.IngressRule {
                {
                    Host: servername,
                    IngressRuleValue: extensions.IngressRuleValue {
                        HTTP: &extensions.HTTPIngressRuleValue {
                        Paths: []extensions.HTTPIngressPath {
                            {
                                Path: "/",
                                Backend: backend,
                            },
                        },
                        },
                    },
                },
            },
        },
    }
}
