package main

import (
	"go.uber.org/zap"
	"secondlive/service_discovery"
	"context"
	"secondlive/pb"
	"secondlive/pbservice"
	"time"
)

func main(){
	test_grpc()
}

func test_grpc(){
	StartClient()
	StartService()
}


func StartService(){
	//zap,etcd,serviceMana,serviceDiscovery,myGrpc
	_,_,_,serviceDiscovery,myGrpc := GetInstance()

	serviceName := "zgoframe"

	ip := "127.0.0.1"
	listenIp := "127.0.0.1"
	port := "6666"

	node := service_discovery.ServiceNode{
		ServiceId: 5,
		ServiceName: serviceName,
		Ip:ip ,
		ListenIp:listenIp,
		Port:port ,
		Protocol: service_discovery.SERVICE_PROTOCOL_GRPC,
		IsSelfReg: true,
	}

	err := serviceDiscovery.Register(node)
	if err != nil{
		service_discovery.ExitPrint("serviceDiscovery Register err:",err)
	}

	MyGrpcServer,err := myGrpc.GetServer(serviceName,node.ListenIp,node.Port)
	if err != nil{
		service_discovery.MyPrint("GetServer err:",err)
		return
	}
	//挂载服务的handler
	pb.RegisterZgoframeServer(MyGrpcServer.GrpcServer, &pbservice.Zgoframe{})
	service_discovery.MyPrint("grpc ServerStart...")
	MyGrpcServer.ServerStart()
	service_discovery.MyPrint("GrpcServer.Serve:",err)

	//结束的时候，记得执行下面的两个关闭方法，不然ETCD里会有一定延迟
	//myGrpc.Shutdown()
	//serviceDiscovery.Shutdown()
	for  {
		time.Sleep(time.Second * 1)
	}
}

func GetInstance()( *zap.Logger, *service_discovery.MyEtcd,*service_discovery.ServiceManager, *service_discovery.ServiceDiscovery, *service_discovery.GrpcManager){
	AppName := "test_zgo"
	AppEnv := "local"
	zap,err   := service_discovery.GetNewZapLog("mylog.log",1)
	if err != nil{
		service_discovery.ExitPrint("GetNewZapLog err:"+err.Error())
	}

	option := service_discovery.EtcdOption{
		AppName		: AppName,
		AppENV		: AppEnv,
		AppKey		: "",
		//FindEtcdUrl : "127.0.0.1",
		Username	: "",
		Password	: "",
		Ip			: "127.0.0.1",
		//Ip 			: "8.142.177.235",
		Port		: "2379",
		Log			: zap,
	}

	etcd ,err := service_discovery.NewMyEtcdSdk(option)


	serviceMana,err := service_discovery.NewServiceManager( )
	if err != nil{
		service_discovery.ExitPrint("GetNewServiceManager err:"+err.Error())
	}

	serviceOption := service_discovery.ServiceDiscoveryOption{
		Log		: zap,
		Etcd	: etcd,
		Prefix	: "/service",
		DiscoveryType: service_discovery.SERVICE_DISCOVERY_ETCD,
		ServiceManager: serviceMana,
	}
	serviceDiscovery ,err := service_discovery.NewServiceDiscovery(serviceOption)
	if err != nil{
		service_discovery.ExitPrint("NewServiceDiscovery err:"+err.Error())
	}


	grpcManagerOption := service_discovery.GrpcManagerOption{
		AppId: 0,
		ServiceId: 5,
		Log: zap,
	}
	myGrpc,err :=  service_discovery.NewGrpcManager(grpcManagerOption)
	if err != nil{
		service_discovery.ExitPrint("NewGrpcManager err:"+err.Error())
	}

	return zap,etcd,serviceMana,serviceDiscovery,myGrpc
}

func  StartClient()error{
	//util.ExitPrint(global.V.ServiceManager.GetByName("zgoframe"))
	//grpcClientConn,err := global.V.Grpc.GetClient(global.C.Grpc.Ip,global.C.Grpc.Port)
	//dns := global.C.Grpc.Ip+ ":4141"
	//dns := global.C.Grpc.Ip+ ":6666"
	//grpcClientConn, err := grpc.Dial(dns,grpc.WithInsecure())
	//serviceName :=  global.V.Service.Name

	_,_,_,serviceDiscovery,myGrpc := GetInstance()

	serviceName:="zgoframe"
	serviceNode ,err := serviceDiscovery.GetLoadBalanceServiceNodeByServiceName(serviceName,"")
	if err != nil{
		service_discovery.ExitPrint("GetServiceNodeByServiceName err:"+err.Error())
	}

	service_discovery.MyPrint("serviceNode:",serviceNode)

	grpcClientConn, err := myGrpc.GetClient(serviceName,0,serviceNode.Ip,serviceNode.Port)
	//grpcClientConn, err := grpc.Dial(dns,grpc.WithInsecure(),grpc.WithUnaryInterceptor(clientInterceptorBack))
	//util.MyPrint("client grp dns:",dns , " err:",err)
	if err != nil{
		service_discovery.MyPrint("grpc GetClient err:",err)
		return  err
	}

	pbServiceFirst := pb.NewZgoframeClient(grpcClientConn)
	RequestRegPlayer := pb.RequestUser{}
	RequestRegPlayer.Id = 123123
	RequestRegPlayer.Nickname = "xiaoz"
	res ,err:= pbServiceFirst.SayHello(context.Background(),&RequestRegPlayer)
	service_discovery.MyPrint("grpc return:",res , " err:",err)


	//global.V.ServiceDiscovery.ShowJsonByService()
	//global.V.ServiceDiscovery.ShowJsonByNodeServer()

	return nil
}

//func client2()error{
//
//	go clientSend()
//
//	return nil
//}
//
//func clientSend(){
//	for{
//		serviceName :=  global.V.Service.Name
//		grpcClientConn, err := global.V.Grpc.GetClientByLoadBalance(serviceName,0)
//		if err != nil{
//			util.MyPrint("grpc GetClient err:",err)
//			return
//		}
//
//		pbServiceFirst := pb.NewZgoframeClient(grpcClientConn)
//		RequestRegPlayer := pb.RequestUser{}
//		RequestRegPlayer.Id = 123123
//		RequestRegPlayer.Nickname = "xiaoz"
//		res ,err:= pbServiceFirst.SayHello(context.Background(),&RequestRegPlayer)
//		util.MyPrint("grpc return:",res , " err:",err)
//
//		time.Sleep(time.Second * 1)
//		util.MyPrint("sleep 1 second...")
//	}
//}