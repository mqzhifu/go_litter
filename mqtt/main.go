package main

import (
	"github.com/eclipse/paho.mqtt.golang"
	"fmt"
	"os"
	"time"
)


func connectionLostHook(client mqtt.Client, err error){
	fmt.Println("connectionLostHook ================")
}

func reconnectHook( ){
	fmt.Println("reconnectHook ================")
}

//当，有消息推送过来，但是没有handler接收时，会走此函数
func receiveNobodyHook(client mqtt.Client, message mqtt.Message){
	fmt.Println("receiveNobodyHook")
}

func systemSubscribeHook(c *CkMqtt, msg mqtt.Message){
	fmt.Println("systemSubscribeHook ================")
}
//以上4个，先定义 成公共的

//用户/前端，创建mqtt连接后的回调函数，这里可以做些日志处理、再订阅等操作
func ConnectedAfterProductHook(c *CkMqtt){
	fmt.Println("ConnectedAfterProductHook")
	//os.Exit(-100)




}
//同上
func ConnectedAfterConsumerHook(c *CkMqtt){
	fmt.Println("ConnectedAfterConsumerHook")
	//消息总线订阅
	//topic string, qos byte, callback mqtt.MessageHandler
	//if token := c.Subscribe("consumer/wdy", 0, consumerReceiveUserHand); token.Wait() && token.Error() != nil {
	//	fmt.Println(token.Error())
	//	os.Exit(1)
	//}
	topic := "serv/product3"
	token ,err := c.Subscribe(topic, 0, consumerReceiveUserHand )
	if err != nil{
		fmt.Println("subscribe err : ",err.Error())
		os.Exit(-100)
	}

	if token.Wait() && token.Error() != nil{
		fmt.Println(token.Error())
		os.Exit(-100)
	}
}

//所有新的连接者，都会用clientId做主题名，创建一个订阅
func productReceiveUserHand(client mqtt.Client, message mqtt.Message){
	fmt.Println("productReceiveUserHand")
}
//某个主题的接收者
func consumerReceiveUserHand(client mqtt.Client, message mqtt.Message){
	fmt.Println("consumerReceiveUserHand ================", string(  message.Payload()))
}

func connect(appId int ,connectedAfterHook ConnectOkCallback){
	//先初始化app项目信息


	//初始化连接参数
	connectMqttData := NewConnectMqttDataStruct()
	connectMqttData.Protocol		= CONN_MQTT_PROTOCOL_TCP //接mqtt broker 的协议
	connectMqttData.CleanSession	= false
	connectMqttData.Order			= true
	connectMqttData.KeepAlive		= 30
	connectMqttData.PingTimeout		= 3
	connectMqttData.ConnectTimeout	= 3
	connectMqttData.AutoReconnect	= false
	connectMqttData.ConnectOkCallback 		= connectedAfterHook
	connectMqttData.ConnectionLostCallback 	= connectionLostHook
	connectMqttData.ReconnectCallback		= reconnectHook
	connectMqttData.UnkonwMessageCallback	= receiveNobodyHook
	connectMqttData.SystemSubscribeCallback	= systemSubscribeHook
	//connectMqttData.AppInfo			= appInfo

	//遗愿消息
	//lastWishMsg := LastWishMsg{
	//	Topic : "aaa",
	//	Qos : 0,
	//	Payload : "aaaa",
	//	Retained : false,
	//}
	//connectMqttData.LastWishMsg = lastWishMsg

	//实例化 sdk
	ckMqtt = NewCkMqtt(appId)
	//获取clientId , 由sdk自动生成
	//clientId := ckMqtt.GenerateRandClientId()
	clientId := ckMqtt.GenerateClientIdByKeyword("z")
	connectMqttData.ClientId =   clientId
	//先初始化一下，连接参数
	ckMqtt.NewClient(connectMqttData)
	//开始连接
	token := ckMqtt.connect()
	if  token.Wait() && token.Error() != nil {
		fmt.Println("NewClient connect err:",token.Error())
		os.Exit(-100)
	}
}

func testDisconnect(){
	time.Sleep(2 * time.Second)
	ckMqtt.Disconnect()
}

//测试 - 订阅接收消息
func testSubscript(){
	var appId = 2
	connect(appId,ConnectedAfterConsumerHook)

	//clientJwt := utils.MakeSuperClientJwt(username, clientId)


	deadLoop()
}



//测试 - 发送消息
func testPublish(){
	var appId = 1
	connect(appId,ConnectedAfterProductHook)
	//先发送100条消息，主题没有人订阅的情况，也就是消息发送出去，会被exmqtt 丢弃的情况
	//for i:=0;i<100;i++{
	//	publishErrorTopicName()
	//}
	//发送完，到emqtt broker 中 检查 dropped.no_subscribers  量


	//发送100条消息，主题存在且之前被订阅过，且cleanSession=false ，但是该订阅者意外下线了
	//得先开启一个订阅者，且cleanSession=false，然后立刻下线
	for i:=0 ; i < 100 ; i++{
		publishTopicExistButSubscriptOffline()
	}
	//发送完，到emqtt broker 中 检查 该session :消息队列 数



	deadLoop()
}

func publishOneMsgAndCheck(topic string, qos byte, retained bool, payload string){
	publishingOneMsg ,err := ckMqtt.Publish(topic,2,retained,payload)
	//先检查基础语法
	if err != nil{
		fmt.Print("ckMqtt.Publish err: ",err.Error())
		os.Exit(-100)
	}
	//os.Exit(-100)
	//开个协程，异步去接收emqtt broker 的返回结果
	go func() {
		//注：这里返回的结果，是broker是否收到了消息，但是，是否投递到后端server是未知的
		_ = publishingOneMsg.Wait()
		if publishingOneMsg.Error() != nil {
			fmt.Println("testPublish happen error:",publishingOneMsg.Error())
		}
		fmt.Println("publish rs:ok ")
	}()
}
//发送一条主题存在且之前被订阅过，且cleanSession=false ，但是该订阅者意外下线了
func publishTopicExistButSubscriptOffline(){
	topic := "serv/product3"
	payload := "hi,im test z! i want test unit : publishTopicExistButSubscriptOffline"

	publishOneMsgAndCheck(topic,0,false,payload)
}
//发送一条主题名错误/主题没有被订阅过 的错误消息
func publishErrorTopicName(){
	errTopic := "user/product1"
	payload := "hi,im test z! i want test unit : testPublishErrorTopicName"

	publishOneMsgAndCheck(errTopic,0,false,payload)
}

func deadLoop(){
	select {

	}
	//for{
	//	if deadLoopSwitch == 0 {
	//		break
	//	}
	//}
}

var deadLoopSwitch = 1
//app 管理类
//var app *App

var ckMqtt *CkMqtt
//主体-测试函数
func main(){
	//待解决问题
	// 1、是否可以给自己订阅的主题~发消息，2、如果可以，那么如果判断死循环？
	// 3、主题名一样，但是clientId不同，消息如何接收？

	//testPublish()
	testSubscript()
}
