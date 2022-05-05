package main

import (
	"github.com/mqzhifu/configcenter"
	"fmt"
	"os"
	"encoding/json"
	"strings"
	"github.com/eclipse/paho.mqtt.golang"
	"log"
	"strconv"
	"time"
	"errors"
)

//配置中心读取的目录 ，这里也可以使用http 方式，省略此配置项
var rooPath = "/data/www/golang/src/configCenter/"
//配置中心 类
var configer * configcenter.Configer

const (
	//连接协议
	CONN_MQTT_PROTOCOL_TCP  = "tcp"
	CONN_MQTT_PROTOCOL_WS  = "ws"
	CONN_MQTT_PROTOCOL_WSS  = "wss"
	CONN_MQTT_PROTOCOL_SSL  = "ssl"

	//配置中心的key值
	CONFIG_USER_KEY_NAME = "user"
	CONFIG_SERV_KEY_NAME = "service"
)
//CkMqtt 类
type CkMqtt struct {
	pahoClientOptions 		*mqtt.ClientOptions	//连接paho mqtt sdk 时，初始化连接时的参数配置
	pahoClientInstance		mqtt.Client			//已创建连接的一个client实例
	//配置中心 读取出来的配置值
	configDataMap 			ConfigData
	msgPaylodMaxSize		int //发送一条消息，消息体最大值，MB
	//所有使用的配置初始化参数，均存在这个结构体里，这是个偷懒的选择
	myConnectMqttData 		ConnectMqttData
	appInfo					DataRecord
	SubscribeTopicMaxNum	int //单个APP可最订阅最多主题数为
}
//用于连接mqtt broker
type ConnectMqttData struct {
	ip 						string
	port 					string
	username 				string
	ps 						string
	protocolIpPort 			string		//"tcp://39.106.65.76:1883"
	connectRetry 			bool		//创建连接失败后，重连
	connectRetryInterval 	time.Duration //创建连接失败后，重试最大周期间隔
	//以上，均是sdk设置，使用者不能初始化
	//	其中IP PORT ，根据APP_ID自动从配置中计算得出，不需要使用者配置
	// 	重连没有放开，是因为会出现死循环，因为内部是goto,且没找到任何回调函数可以中断

	ClientId 				string		//客户端唯一标识，如果重复，后连接的会踢掉前一个连接，建议由SDK自动生成
	Protocol 				string		//tcp ws wss ssl
	LastWishMsg 			LastWishMsg	//遗愿
	CleanSession 			bool		//是否持久化session
	Order					bool		//未知，文档说：是否顺序发送, true: 同步顺序发送消息, false: 异步发送消息. 发送的消息有可能乱序
	ResumeSubs				bool		//当cleanSession=false 时，qos > 0 时，有些消息，是可以恢复

	KeepAlive				int			//保持长连接心跳时间，0为没有心跳保持
	PingTimeout				int			//当开启keepAlive后，定期会有ping 心中包，超时时间
	ConnectTimeout			int			//client 连接broker 超时时间


	AutoReconnect			bool		//当连接丢失时，自动重连
	MaxReconnectInterval	time.Duration //当连接丢失时，两次重试连接的，最大间隔周期

	//AppInfo 				DataRecord	//这个是该应用的一些描述信息，用于生成clientId、类型区分

	//下面5个是，callback 用户态的几个回调函数
	ConnectOkCallback  		ConnectOkCallback		//连接成功后-回调
	ConnectionLostCallback	ConnectionLostCallback 	//连接意外丢失-回调,但：Disconnect、ForceDisconnect 不会执行回调。
	ReconnectCallback		ReconnectCallback		//重连接-回调,初始连接丢失后，在重新连接之前调用
	UnkonwMessageCallback	UnkonwMessageCallback 	//目前不确定这个回调，文档说：发布的消息，订阅者已接收(接收到的消息未知)
	//以上4个是，mqttSDK自带的，下面这个是，ckmqtt SDK自带的
	SystemSubscribeCallback	SystemSubscribeCallback //系统默认自定义的主要是，有消息来时，回调

	//Store mqtt.Store
	//Servers
	//WriteTimeout
	//HTTPHeaders
	//WebsocketOptions
}
//获取一个，创建连接的初始化参数的结构体，主要用于默认值设置
func NewConnectMqttDataStruct()ConnectMqttData{
	connectMqttData := ConnectMqttData{
		Protocol		: CONN_MQTT_PROTOCOL_TCP,
		CleanSession	: true,
		Order			: true,
		KeepAlive		: 30,
		PingTimeout		: 3,
		ConnectTimeout	: 3,
		AutoReconnect	: false,
		MaxReconnectInterval	: 10 * time.Minute,

		connectRetryInterval:    30 * time.Second,
		connectRetry	: false,
		ResumeSubs		: false,
	}
	return connectMqttData
}
//遗愿消息体结构
type LastWishMsg struct {
	Topic string
	Qos byte
	Payload string
	Retained bool
}
//包含 所有从配置文件中读取出来的 配置值
type ConfigData struct {
	Host   		map[string]map[string]string
	Acl   		map[string]map[string]string
	Topic   	map[string]map[string]string
	//以上3个是直接从配置文件中读取的，未处理过的信息

	Topic_base   map[string]map[string]string
	TopicList	map[string]map[string][]string //从上面的Topic中取出 所有-主题/列表,1格式化成 数组 ，2 校验主题名是否合法
}
//以下上，各种回调函数的定义
type ConnectOkCallback 			func(*CkMqtt)	//连接成功后
type ConnectionLostCallback	func(client mqtt.Client, err error)				//连接丢失后
type ReconnectCallback	func()					//连接丢失后，进入重连接前
type UnkonwMessageCallback func(client mqtt.Client, message mqtt.Message)				//收到未知消息的回调
//sdk自动以<系统>身份订阅的主题，有消息过来时，回调函数
type SystemSubscribeCallback 	func(*CkMqtt, mqtt.Message)


//=======================以上所有初始化信息结束，下面是执行方法了================================

//实例化-类，构造函数
func NewCkMqtt(appId int) *CkMqtt{
	ckMqtt := new (CkMqtt)

	mqtt.ERROR = log.New(os.Stdout, "[ERROR] ", 0)
	mqtt.CRITICAL = log.New(os.Stdout, "[CRIT] ", 0)
	mqtt.WARN = log.New(os.Stdout, "[WARN]  ", 0)
	mqtt.DEBUG = log.New(os.Stdout, "[DEBUG] ", 0)


	ckMqtt.msgPaylodMaxSize = 2	//消息体最大2MB

	app := NewApp()
	ckMqtt.appInfo = app.GetById(appId)
	ckMqtt.initConfigContainer()
	return ckMqtt
}
//初始化配置
func (ckMqtt *CkMqtt) initConfig(){
	ip := getIpByLBRand(ckMqtt.configDataMap.Host)//随机：从所有IP中取一个
	port := ckMqtt.configDataMap.Host[ip][ckMqtt.myConnectMqttData.Protocol]//每个IP下面的不同协议，有不同的端口号，获取一个
	//fmt.Println("ip:",ip ,"port:",port)

	//appRecord := app.GetById(appId)//根据AID 读取出 app 信息

	userMap := ckMqtt.configDataMap.Acl[ip]//找出，该APPID 使用哪个用户登陆

	connectUserPsStr := userMap[ckMqtt.getKeyByAppType(ckMqtt.appInfo.Type)]
	connectUserPs := strings.Split(connectUserPsStr,",")
	//fmt.Println(connectUserPs)

	ckMqtt.myConnectMqttData.port = port
	ckMqtt.myConnectMqttData.ip = ip
	ckMqtt.myConnectMqttData.Protocol = ckMqtt.myConnectMqttData.Protocol
	ckMqtt.myConnectMqttData.protocolIpPort = ckMqtt.myConnectMqttData.Protocol + "://" + ip + ":" + port
	ckMqtt.myConnectMqttData.ps = strings.TrimSpace(connectUserPs[1])
	ckMqtt.myConnectMqttData.username = strings.TrimSpace(connectUserPs[0])

	//fmt.Print(myConnectMqttData)
}
//初始化配置中心
func (ckMqtt *CkMqtt)  initConfigContainer(){
	fileTotalSizeMax := 100
	fileSizeMax := 2
	fileCntMax :=100
	allowExtType := "ini"
	configer = configcenter.NewConfiger(fileTotalSizeMax,fileSizeMax,fileCntMax,allowExtType)
	//让配置器，读取目录下的，所有配置文件，加载到内存
	configer.StartLoading(rooPath)
	//根据appId 取到该app-name
	//appRecord := app.GetById(appId)
	//配置中心的分层维度是以项目名为一个文件夹子~一个项目有N多配置文件
	//读取出该项目下的所有配置文件
	appConfigData ,err := configer.Search(ckMqtt.appInfo.Name)
	if err != nil{
		fmt.Print(err.Error())
		os.Exit(-100)
	}
	//配置中心读取出来的内容是JSON ，再转一下，存到本地内存中
	ckMqtt.configDataMap = ConfigData{}
	json.Unmarshal( []byte(appConfigData) , &ckMqtt.configDataMap  )


	myTopicList := make(map[string]map[string][]string)
	for role ,appTopics :=  range ckMqtt.configDataMap.Topic{
		tmp := make(map[string][]string)
		//println(appTopics,role)
		for appId , topics := range appTopics {
			fmt.Println("foreach : ",role , appId , topics)
			tmp[appId] = ckMqtt.loadTopicListAndCheckAndReplace(topics)

		}
		myTopicList[role] = tmp
	}
	ckMqtt.configDataMap.TopicList = myTopicList
	//fmt.Println(ckMqtt.configDataMap.TopicList)
	//os.Exit(-100)

}
//检测多个主题名前缀是否正确，
func  (ckMqtt *CkMqtt) checkTopicsArrPrefix (topics []string,appType int){
	for i := 0;i<len(topics);i++ {
		ckMqtt.checkTopicPrefix(topics[i],appType)
	}
}
//检测一个主题名前缀是否正确，前端/用户 的前缀是user开头，后端/服务 是serv开头
func (ckMqtt *CkMqtt) checkTopicPrefix(topic string,appType int){
	prefix :=  ckMqtt.configDataMap.Topic_base["subscript_prefix"][ckMqtt.getKeyByAppType(appType)]
	index := strings.Index(topic, prefix)
	if index == -1 || index != 0 {
		fmt.Println("主题名需要包含前缀:"+prefix+ " ("+topic+")" )
		os.Exit(-100)
	}
}
//读取配置文件中的json格式的主题名，转换成数组列表，并检查主题名是否有特殊字符，同时再把动态变量替换掉
func (ckMqtt *CkMqtt) loadTopicListAndCheckAndReplace(topics string)[]string{
	fmt.Println("loadTopicListAndCheckAndReplace")
	topicsArr := strings.Split(topics,",")
	for i:=0;i<len(topicsArr);i++{
		topicsArr[i] = strings.Replace(topicsArr[i], "{clientId}", ckMqtt.myConnectMqttData.ClientId, -1)
		checkName := CheckTopicName(topicsArr[i])
		if checkName != "ok"{
			fmt.Println("CheckTopicName err : "+checkName)
			os.Exit(-100)
		}
	}
	return topicsArr
}
//配置文件中 前端/后端  键  值，动态获取
func(ckMqtt *CkMqtt) getKeyByAppType(appType int)string{
	var key string
	if appType == TYPE_USER{//前端用户
		return CONFIG_USER_KEY_NAME
	}else{//service 后端用户
		return CONFIG_SERV_KEY_NAME
	}

	return key
}
//生成一个随机-clientId 尽量短小，因为消息体内包含clientId ,用来给 消费者 回传信息
func (ckMqtt *CkMqtt) GenerateRandClientId( )string {
	dataRecord := ckMqtt.appInfo
	clientId := ckMqtt.configDataMap.Topic_base["client_id_prefix"][ckMqtt.getKeyByAppType(dataRecord.Type)]+strconv.Itoa(dataRecord.Id) +"_"+ strconv.Itoa(getRandIntNum(100))

	return clientId
}
//生成一个clientId，根据关键词，这种是固定clientId
func (ckMqtt *CkMqtt) GenerateClientIdByKeyword(keyword string)string {
	dataRecord := ckMqtt.appInfo
	clientId := ckMqtt.configDataMap.Topic_base["client_id_prefix"][ckMqtt.getKeyByAppType(dataRecord.Type)]+strconv.Itoa(dataRecord.Id) + "_"+keyword

	return clientId
}
//统一管理所有主题名称，也就是加前缀
//func (ckMqtt *CkMqtt) GenerateTopicName(topic string)string {
//	dataRecord := ckMqtt.myConnectMqttData.AppInfo
//	topicName := ckMqtt.configDataMap.Topic_base["subscript_prefix"][ckMqtt.getKeyByAppType(dataRecord.Type)] + topic
//	return topicName
//}
//主题名需要过滤掉一些特殊字符
func CheckTopicName(topic string)string{
	//"#","+" 通配符
	//{} :用于动态变量替换
	//$:这个开头的是emqx-broker的内部系统主题名
	specialChar := [5]string{"#","+", "{","}", "$"}
	for i:=0;i<len(specialChar);i++{
		index := strings.Index(topic, specialChar[i])
		if index != -1{
			fmt.Println("主题名(",topic,")包含特殊字符:"+specialChar[i])
			return specialChar[i]
		}
	}
	return "ok"
}
//检查 主题名 是否在配置文件中有声明
//myType 1:订阅 2发布消息
func  (ckMqtt *CkMqtt)checkTopicDefined(topic string,myType int )(bool,error){
	var topicList []string
	if myType == 1 {
		//订阅类型的，只能订阅自己配置文件中的主题
		topicList = ckMqtt.configDataMap.TopicList[ckMqtt.getKeyByAppType(ckMqtt.appInfo.Type)][strconv.Itoa(ckMqtt.appInfo.Id)]
	}else{
		//发布主题，则可以给配置文件中所有的主题发消息
		//将3维MAP 转成 一维 数组
		for _, appTopic := range ckMqtt.configDataMap.TopicList {
			for _ , oneTopicArr := range appTopic{
				for _,oneTopic :=  range oneTopicArr{
					topicList = append(topicList,oneTopic)
				}
			}
		}
	}
	if len(topicList) == 0{
		return false,errors.New("配置文件中:未声明任何主题")
	}

	for _, eachItem := range topicList {
		if eachItem == topic {
			return true,nil
		}
	}
	return false,errors.New("该主题名："+topic+"，未在配置文件中声明")
}
////合并两个 特殊  map 类型
//func mapMerge(map1 map[string]map[string][]string,map2 map[string]map[string][]string){
//
//}

func (ckMqtt *CkMqtt) NewClient(connectMqttData ConnectMqttData)mqtt.Client{
	ckMqtt.myConnectMqttData = connectMqttData
	//fmt.Println(connectMqttData)
	ckMqtt.initConfig()//根据APPID 初始化   ip port user ps
	fmt.Println(ckMqtt.myConnectMqttData)
	//os.Exit(-10)


	ckMqtt.pahoClientOptions = mqtt.NewClientOptions().
		AddBroker(ckMqtt.myConnectMqttData.protocolIpPort).
		SetClientID(ckMqtt.myConnectMqttData.ClientId).
		SetUsername(ckMqtt.myConnectMqttData.username).
		SetPassword(ckMqtt.myConnectMqttData.ps).


		SetCleanSession(ckMqtt.myConnectMqttData.CleanSession).
		SetAutoReconnect(ckMqtt.myConnectMqttData.AutoReconnect).

		SetConnectTimeout(time.Duration(ckMqtt.myConnectMqttData.ConnectTimeout)  * time.Second).
		SetKeepAlive(time.Duration(ckMqtt.myConnectMqttData.KeepAlive) * time.Second).
		SetPingTimeout(time.Duration(ckMqtt.myConnectMqttData.PingTimeout) * time.Second).

		SetConnectRetry(ckMqtt.myConnectMqttData.connectRetry).
		SetConnectRetryInterval(ckMqtt.myConnectMqttData.connectRetryInterval).

		SetOrderMatters(ckMqtt.myConnectMqttData.Order).

		SetOnConnectHandler(ckMqtt.OnConnectHandler).
		SetReconnectingHandler(ckMqtt.ReconnectHandler).
		SetConnectionLostHandler(ckMqtt.ConnectionLostHandler).
		SetDefaultPublishHandler(ckMqtt.DefaultPublishMessageHandler)

	//var testEmptyWill = LastWishMsg{}
	if (LastWishMsg{}) == ckMqtt.myConnectMqttData.LastWishMsg{
		fmt.Println(" LastWishMsg struct is empty")
	}else{
		ckMqtt.pahoClientOptions.SetWill(
			ckMqtt.myConnectMqttData.LastWishMsg.Topic,
			ckMqtt.myConnectMqttData.LastWishMsg.Payload,
			ckMqtt.myConnectMqttData.LastWishMsg.Qos,
			ckMqtt.myConnectMqttData.LastWishMsg.Retained)
	}

	//SetDefaultPublishHandler(productReceiveNobodyHand)

	ckMqtt.pahoClientInstance = mqtt.NewClient(ckMqtt.pahoClientOptions)
	return ckMqtt.pahoClientInstance
}

func  (ckMqtt *CkMqtt) connect() mqtt.Token{
	return ckMqtt.pahoClientInstance.Connect()
}

func  (ckMqtt *CkMqtt) Disconnect(){
	//关闭连接时，等待1秒钟，给一些未结束的协程最后处理时间
	ckMqtt.pahoClientInstance.Disconnect(1000)
}

//======以下4个，handler结尾的，都是 mqtt 回调的
func  (ckMqtt *CkMqtt) ConnectionLostHandler(client mqtt.Client, err error){
	fmt.Println("ConnectionLostHandler")
	if(ckMqtt.myConnectMqttData.ConnectionLostCallback != nil){
		ckMqtt.myConnectMqttData.ConnectionLostCallback(client,err)
	}
}

func  (ckMqtt *CkMqtt) ReconnectHandler(client mqtt.Client, ckuebtIotuibs *mqtt.ClientOptions ){
	fmt.Println("ReconnectHandler")
	if(ckMqtt.myConnectMqttData.ReconnectCallback != nil){
		ckMqtt.myConnectMqttData.ReconnectCallback()
	}
}

func  (ckMqtt *CkMqtt) DefaultPublishMessageHandler(client mqtt.Client, message mqtt.Message){
	fmt.Println("DefaultPublishMessageHandler")
	if(ckMqtt.myConnectMqttData.UnkonwMessageCallback != nil){
		ckMqtt.myConnectMqttData.UnkonwMessageCallback(client,message)
	}
}
//连接成功后，回调函数
//这里，主要是把  系统-默认主题，都给订阅上
func  (ckMqtt *CkMqtt) OnConnectHandler(c mqtt.Client){
	fmt.Println("OnConnectHandler")

	//1、先处理，配置文件中，默认需要订阅的主题
	topicsKEY := ""
	dataRecord := ckMqtt.appInfo
	if dataRecord.Type == TYPE_USER{//前端用户
		topicsKEY = CONFIG_USER_KEY_NAME
	}else{//service 后端用户
		topicsKEY = CONFIG_SERV_KEY_NAME
	}
	topics,err := ckMqtt.configDataMap.Topic_base["default_subscript"][topicsKEY]
	fmt.Println(ckMqtt.configDataMap.Topic_base)
	if !err {
		fmt.Println("default_subscript is null...")
		return
	}

	topicsArr := strings.Split(topics,",")
	for i:=0;i<len(topicsArr);i++{
		topicsArr[i] = strings.Replace(topicsArr[i], "{clientId}", ckMqtt.myConnectMqttData.ClientId, -1)
		//checkName := CheckTopicName(topicsArr[i])
		//if checkName != "ok"{
		//	fmt.Println("CheckTopicName err : "+checkName)
		//	os.Exit(-100)
		//}
	}
	//topicsArr := ckMqtt.loadTopicListAndCheckAndReplace(topics)
	//fmt.Println(topicsArr)
	//os.Exit(-100)
	for i:=0;i<len(topicsArr);i++{
		fmt.Println(topicsArr[i])
		ckMqtt.subscribeSystem(topicsArr[i],0,ckMqtt.SystemSubscribeCallback)
	}

	//os.Exit(-100)

	if(ckMqtt.myConnectMqttData.ConnectOkCallback != nil){
		ckMqtt.myConnectMqttData.ConnectOkCallback(ckMqtt)
	}

}

func (ckMqtt *CkMqtt)SystemSubscribeCallback(c mqtt.Client, message mqtt.Message){
	//configDataMap.Topic_base["client_id_prefix"]["user"]
	fmt.Println("SystemSubscribeCallback : ",message)
	if ckMqtt.myConnectMqttData.SystemSubscribeCallback != nil{
		ckMqtt.myConnectMqttData.SystemSubscribeCallback(ckMqtt,message)
	}

}

//==========================

//系统默认订阅主题 - 与普通主题订阅的区别是，不做check
func (ckMqtt *CkMqtt) subscribeSystem(topic string, qos byte, callback mqtt.MessageHandler)(mqtt.Token,error){
	fmt.Println("subscribeSystem topic : ",topic)
	Token := ckMqtt.pahoClientInstance.Subscribe(topic,qos,callback)
	return Token,nil
}
//订阅一个主题
func (ckMqtt *CkMqtt) Subscribe(topic string, qos byte, callback mqtt.MessageHandler)(mqtt.Token,error){
	checkName := CheckTopicName(topic)
	if checkName != "ok"{
		return nil,errors.New("CheckTopicName err : "+checkName)
	}

	_,err := ckMqtt.checkTopicDefined(topic,1)
	if err != nil{
		fmt.Println("checkTopicDefined err:",err.Error())
		return nil,errors.New(err.Error())
	}

	ckMqtt.checkTopicPrefix(topic,ckMqtt.appInfo.Type)
	fmt.Println("Subscribe topic : ",topic)
	//topic = ckMqtt.GenerateTopicName(topic)
	Token := ckMqtt.pahoClientInstance.Subscribe(topic,qos,callback)
	return Token,nil
}
//给某主题发送一条消息
func (ckMqtt *CkMqtt) Publish(topic string, qos byte, retained bool, payload string)(mqtt.Token,error){
	fmt.Println("Publish msg , topic=",topic," isSystemTopic ： ",isSystemTopic(topic))
	if !isSystemTopic(topic){//系统自定义的主题，没有前缀
		//非系统的主题，需要给加上service 前缀
		//topic = ckMqtt.configDataMap.Topic_base["subscript_prefix"][CONFIG_SERV_KEY_NAME] + topic

		//检查主题名是否在配置中有定义
		_,err := ckMqtt.checkTopicDefined(topic,2)
		if err !=nil {
			println("checkTopicDefined:",err.Error())
			return nil,err
		}
	}

	payloadLenMB := len(payload) / 1024 / 1024
	if payloadLenMB > ckMqtt.msgPaylodMaxSize{//最大2MB
		return nil,errors.New("消息内容，最大不能超过2mb")
	}

	Token := ckMqtt.pahoClientInstance.Publish(topic,qos,retained,payload)
	return Token,nil
}
//负载获取一个IP，随机算法
//只是个权宜之计，按说应该走consul zk etcd
//另外，如果某些业务场景，需要同一ID永久分配固定IP（服务器没挂掉），还得换算法
func getIpByLBRand(ipList map[string]map[string]string)string{
	totalLen := len(ipList)
	randNum := getRandIntNum(totalLen )
	//fmt.Println("totalLen",totalLen,"randNum",randNum)
	inc := 0
	for key,_ := range ipList {
		if inc == randNum{
			return key
		}
		inc++
	}
	return ""
}

func isSystemTopic(topic string) bool{
	specialChar := [2]string{"sys", "$queue"}
	for i:=0;i<len(specialChar);i++ {
		index := strings.Index(topic, specialChar[i])
		if index == 0 {
			return true
		}
	}
	return false
}
