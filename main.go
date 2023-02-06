package main

import (
	"fmt"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"time"
)

var (
	err            error
	config         *rest.Config
	kubeConfigPath string
)

func main() {
	kubeConfigPath = fmt.Sprintf("%s%s", os.Getenv("HOME"), "/.kube/config")

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	// 初始化 informer factory（为了测试方便这里设置每30s重新 List 一次） //这个里的同步是resync不是relist 是吧本地的缓存放在我们的infomer当中来 30秒一次
	//本地缓存是Reflector去list & watch kubernetes api来获取的，存放本地队列中Delta Fifo queue(生产者消费者模型)
	// list 全量同步拉取一次， watch 会持续监听
	informerFactory := informers.NewSharedInformerFactory(clientset, time.Second*30)
	// 对 Deployment 监听
	deployInfomer := informerFactory.Apps().V1().Deployments()
	// 创建 Informer（相当于注册到工厂中去，这样下面启动的时候就会去 List(List所有资源对象缓存到本地 再watch需要watch的资源对象) & Watch 对应的资源）
	informer := deployInfomer.Informer()
	// 创建 Lister 同步缓存
	deployLister := deployInfomer.Lister()
	// 注册事件处理程序 同步缓存事件
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    onAdd,
		UpdateFunc: onUpdate,
		DeleteFunc: onDelete,
	})
	stopper := make(chan struct{})
	defer close(stopper)

	// 启动 informer，List & Watch
	informerFactory.Start(stopper)
	// 等待所有启动的 Informer 的缓存被同步
	informerFactory.WaitForCacheSync(stopper)

	// 从本地缓存中获取 default 中的所有 deployment 列表
	deployments, err := deployLister.Deployments("kube-system").List(labels.Everything())
	if err != nil {
		log.Fatal(err)
	}
	for idx, deploy := range deployments {
		fmt.Printf("%d -> %s\n", idx+1, deploy.Name)
	}
	<-stopper
}

func onAdd(obj interface{}) {
	deploy := obj.(*v1.Deployment) //断言 是否是deployment类型
	fmt.Println("add a deployment:", deploy.Name)
}

func onUpdate(old, new interface{}) {
	oldDeploy := old.(*v1.Deployment) //断言 是否是deployment类型
	newDeploy := new.(*v1.Deployment)
	fmt.Println("update deployment:", oldDeploy.Name, newDeploy.Name)
}

func onDelete(obj interface{}) {
	deploy := obj.(*v1.Deployment) //断言 是否是deployment类型
	fmt.Println("delete a deployment:", deploy.Name)
}
