package gamematch

import (
	"encoding/json"
	"errors"
	"github.com/gomodule/redigo/redis"
	"strconv"
	"strings"
	"sync"
	"zlib"
)

//推送给3方，支持重试
type PushElement struct {
	Id  		int
	ATime 		int
	UTime   	int		//最后更新的时间
	LinkId		int
	Status  	int 	//状态：1未推送2推送失败，等待重试3推送成功4推送失败，不再重试
	Times  		int		//已推送次数
	Category	int		//1:报名超时 2匹配成功 3成功结果超时
	Payload 	string	//自定义的载体
}

type Push struct {
	Mutex 		sync.Mutex
	Rule 		Rule
	//queueSign 	*QueueSign
	QueueSuccess	*QueueSuccess
	Log 		*zlib.Log
	Gamematch	*Gamematch
}

func NewPush(rule Rule,gamematch *Gamematch)*Push{
	push := new(Push)
	push.Rule = rule


	//push.queueSign = NewQueueSign(rule)
	//这里有个问题，循环 new
	//push.QueueSuccess = NewQueueSuccess(rule,push)
	//push.QueueSuccess = gamematch.getContainerSuccessByRuleId(rule.Id)
	push.Log = getRuleModuleLogInc(rule.CategoryKey,"push")
	push.Gamematch = gamematch
	return push
}

//var PushRetryPeriod = []int{10,30,60,600}
//方便测试
var PushRetryPeriod = []int{5,10,15}
//成功类的整个：大前缀
func (push *Push)  getRedisPrefixKey( )string{
	return redisPrefix + redisSeparation + "push"
}
//不同的匹配池(规则)，要有不同的KEY
func (push *Push)  getRedisCatePrefixKey(  )string{
	return push.getRedisPrefixKey() + redisSeparation + push.Rule.CategoryKey
}
func (push *Push)  getRedisPushIncKey()string{
	return push.getRedisCatePrefixKey()  + redisSeparation + "inc_id"
}
func (push *Push) GetPushIncId( )  int{
	key := push.getRedisPushIncKey()
	res,_ := redis.Int(myredis.RedisDo("INCR",key))
	return res
}

func (push *Push) getRedisKeyPushStatus()string{
	return push.getRedisCatePrefixKey() + redisSeparation + "status"
}

//func (push *Push) getRedisKeyPushPrefix()string{
//	return push.getRedisCatePrefixKey() + redisSeparation + "push"
//}

func (push *Push) getRedisKeyPush( id int)string{
	return push.getRedisCatePrefixKey()   + redisSeparation + strconv.Itoa(id)
}


func (push *Push) getById (id int) (element PushElement) {
	key := push.getRedisKeyPush(id)
	res,_ := redis.String(myredis.RedisDo("get",key))
	if res == ""{
		return element
	}

	element = push.pushStrToStruct(res)
	return element
}

func  (push *Push) addOnePush (redisConn redis.Conn,linkId int,category int ,payload string) int {
	mylog.Debug("addOnePush" , linkId,category,payload)
	id :=  push.GetPushIncId()
	key := push.getRedisKeyPush(id)
	pushElement := PushElement{
		Id : id,
		ATime: zlib.GetNowTimeSecondToInt(),
		Status: 1,
		UTime: zlib.GetNowTimeSecondToInt(),
		Times:0,
		LinkId: linkId,
		Category : category,
		Payload : payload,
	}
	pushStr := push.pushStructToStr(pushElement)
	res,err := myredis.Send(redisConn,"set",redis.Args{}.Add(key).Add(pushStr)...)
	//res,err := myredis.RedisDo("set",redis.Args{}.Add(key).Add(pushStr)...)
	mylog.Info("addOnePush rs : ",res,err)
	push.Log.Info("addOnePush ,cate : ",category , "payload:",payload)

	pushKey := push.getRedisKeyPushStatus()
	res,err = myredis.Send(redisConn,"zadd",redis.Args{}.Add(pushKey).Add(PushStatusWait).Add(id)...)
	//res,err = myredis.RedisDo("zadd",redis.Args{}.Add(pushKey).Add(PushStatusWait).Add(id)...)
	mylog.Info("addOnePush status : ",res,err)
	push.Log.Info("addOnePush status")

	return id
}

func (push *Push)  pushStrToStruct(redisStr string )PushElement{
	strArr := strings.Split(redisStr,separation)
	result := PushElement{
		Id:zlib.Atoi(strArr[0]),
		LinkId 	:	zlib.Atoi(strArr[1]),
		ATime 	:	zlib.Atoi(strArr[2]),
		Status : zlib.Atoi(strArr[3]),
		UTime : zlib.Atoi(strArr[4]),
		Times :  zlib.Atoi(strArr[5]),
		Category: zlib.Atoi(strArr[6]),
		Payload :  strArr[7],
	}
	return result
}

func (push *Push) pushStructToStr(pushElement PushElement)string{
	str :=
		strconv.Itoa(pushElement.Id) + separation +
		strconv.Itoa(pushElement.LinkId) + separation +
		strconv.Itoa(pushElement.ATime) + separation +
		strconv.Itoa(pushElement.Status) + separation +
		strconv.Itoa(pushElement.UTime) + separation +
		strconv.Itoa(pushElement.Times) + separation +
		strconv.Itoa(pushElement.Category) + separation +
			pushElement.Payload + separation

	return str
}

//func (push *Push)   delAll(){
//	key := push.getRedisPrefixKey()
//	myredis.RedisDo("del",key)
//}

func (push *Push)   delOneRule(){
	mylog.Debug(" push delOneRule : ",)
	key := push.getRedisCatePrefixKey()+ "*"
	myredis.RedisDelAllByPrefix(key)
	//push.delAllPush()
	//push.delAllStatus()
}

//func  (push *Push)  delAllPush( ){
//	prefix := push.getRedisCatePrefixKey()
//	res,_ := redis.Strings( myredis.RedisDo("keys",prefix + "*"  ))
//	if len(res) == 0{
//		mylog.Notice(" GroupElement by keys(*) : is empty")
//		return
//	}
//	//zlib.ExitPrint(res,-200)
//	for _,v := range res{
//		res,_ := redis.Int(myredis.RedisDo("del",v))
//		zlib.MyPrint("del group element v :",res)
//	}
//}
//
//func  (push *Push)  delAllStatus( ){
//	key := push.getRedisKeyPushStatus()
//	res,_ := redis.Strings( myredis.RedisDo("del",key ))
//	mylog.Debug("delAllStatus :",res)
//}


//PushMatchSuccessOk		int	//推送：匹配成功记录-对方正常接收
//PushMatchSuccessDrop	int//推送：匹配成功记录-对方接收失败
//
//PushMatchSuccessTimeoutOk 	int//推送：匹配成功记录，但对方一直拒绝接收，PUSH也没有超过重度次数，记录本身超时了
//PushMatchSuccessTimeoutDrop int//
//
//PushSignOk		int	//推送：报名超时记录-对方正常接收
//PushSignDrop	int	//推送：报名超时记录-对方接收失败

func  (push *Push)  delOneStatus( redisConn redis.Conn, pushId int){
	key := push.getRedisKeyPushStatus()
	res,err :=  myredis.Send(redisConn,"ZREM",redis.Args{}.Add(key).Add(pushId)... )
	//res,err :=  myredis.RedisDo("ZREM",redis.Args{}.Add(key).Add(pushId)... )
	mylog.Debug(" delOne PushStatus index res",res,err)
	push.Log.Info(" delOne PushStatus index res",res,err)
}
func  (push *Push)metrics(elementCategory int ,action string){
	actionStr := action
	if action == "Ok" || action == "Drop"{

	}else{
		mylog.Error("push metrics action err:",action)
		return
	}

	key := ""
	if elementCategory == PushCategorySignTimeout{
		key = "PushSign" + actionStr
	}else if elementCategory == PushCategorySuccess{
		key = "PushMatchSuccess" + actionStr
	}else if elementCategory == PushCategorySuccessTimeout{
		key = "PushMatchSuccessTimeout" + actionStr
	}else{
		mylog.Error("push metrics category err:",elementCategory)
		return
	}

	mymetrics.FastLog(key,zlib.METRICS_OPT_INC,0)
}
//失败且需要重试的PUSH-ELEMENT
func  (push *Push)  upRetryPushInfo(element PushElement ){
	redisConnFD := myredis.GetNewConnFromPool()
	defer redisConnFD.Close()

	myredis.Send(redisConnFD,"multi")
	element.Status = PushStatusRetry
	element.UTime = zlib.GetNowTimeSecondToInt()
	element.Times = element.Times + 1
	key := push.getRedisKeyPush(element.Id)
	pushStr := push.pushStructToStr(element)
	res,err := myredis.Send(redisConnFD,"set",redis.Args{}.Add(key).Add(pushStr)...)
	//res,err := myredis.RedisDo("set",redis.Args{}.Add(key).Add(pushStr)...)

	push.Log.Info("upRetryPushElementInfo , ", element)
	//这里有个麻烦点，元素信息 和 索引信息，是分开放的，元素的变更比较简单，索引是一个集合，改起来有点麻烦
	//那就直接先删了，再重新添加一条
	statuskey := push.getRedisKeyPushStatus()
	push.Log.Info("del pushStatus index ,pushId : ",element.Id)
	push.delOneStatus(redisConnFD,element.Id)
	res,err = myredis.Send(redisConnFD,"zadd",redis.Args{}.Add(statuskey).Add(PushStatusRetry).Add(element.Id)...)
	//res,err = myredis.RedisDo("zadd",redis.Args{}.Add(statuskey).Add(PushStatusRetry).Add(element.Id)...)

	push.Log.Info("add  new pushStatus index : ",res,err)
	mylog.Info("add  new pushStatus index : ",res,err)

	myredis.Exec(redisConnFD )
}
//在业务里，删除一条push
//走到里，前置条件肯定是PUSH成功了
func  (push *Push) delOneByIdInBusiness(redisConn redis.Conn,id int){
	myredis.Send(redisConn,"multi")
	element := push.getById(id)
	push.delOnePush(redisConn,id)
	if element.Category == PushCategorySuccess || element.Category == PushCategorySuccessTimeout{
		push.Log.Info("delOneResult")
		success := push.Gamematch.getContainerSuccessByRuleId(push.Rule.Id)
		success.delOneResult(redisConn,element.LinkId,1,1,1,1)
	}
	push.metrics(element.Category,"Ok")
	myredis.ConnDo(redisConn,"exec")
}
func  (push *Push)  checkStatus(){
	//mylog.Info("one rule checkStatus : start ")
	//push.Log.Info("one rule checkStatus : start ")
	key := push.getRedisKeyPushStatus()

	push.checkOneByStatus(key,PushStatusWait)
	push.checkOneByStatus(key,PushStatusRetry)
	//push.Log.Info("one rule checkStatus : finish ")

}

func (push *Push)getAllCnt()int{
	key := push.getRedisKeyPushStatus()
	res,_ := redis.Int(  myredis.RedisDo("ZCOUNT",redis.Args{}.Add(key).Add("-inf").Add("+inf")...))
	return res
}

func (push *Push)getStatusCnt(status int)int{
	key := push.getRedisKeyPushStatus()
	res,_ := redis.Int(  myredis.RedisDo("ZCOUNT",redis.Args{}.Add(key).Add(status).Add(status)...))
	return res
}
//status:待推送、重试推送
func  (push *Push)  checkOneByStatus(key string,status int){
	//mylog.Info("checkOneByStatus :",status)
	res,err := redis.Ints(  myredis.RedisDo("ZREVRANGEBYSCORE",redis.Args{}.Add(key).Add(status).Add(status)...))

	if err != nil{
		mylog.Error("redis keys err :",err.Error())
		push.Log.Error("redis keys err :",err.Error())
		return
	}
	now :=zlib.GetNowTimeSecondToInt()
	if len(res) == 0{
		//mylog.Notice(" empty , no need process")
		if now % 10 == 0{
			push.Log.Info("checkOneByStatus :",status)
			push.Log.Notice("checkOneByStatus empty , no need process")
		}
		return
	}
	push.Log.Info("push need process element total : ",len(res))
	for _,id := range res{
		push.processOne(id,status)
	}
}

func  (push *Push) processOne(id int,status int){
	//mylog.Info(" action hook , push id : " ,id ," status : ",status)
	push.Log.Info(" action processOne , push id : " ,id ," status : ",status)
	element := push.getById(id)
	//fmt.Printf("%+v", element)
	if status == PushStatusWait{
		push.Log.Info("element first push")
		push.pushAndUpInfo(element,PushStatusRetry)
	}else{
		push.Log.Info("element retry ,element.Times:",element.Times, " len(PushRetryPeriod):",len(PushRetryPeriod))
		if element.Times >= len(PushRetryPeriod) {
			//已超过，最大重试次数
			push.metrics(element.Category,"Drop")

			redisConnFD := myredis.GetNewConnFromPool()
			defer redisConnFD.Close()

			mylog.Notice(" push retry time > maxRetryTime , drop this msg.")
			push.delOneByIdInBusiness(redisConnFD,id)
		}else{
			time := PushRetryPeriod[element.Times]
			push.Log.Info("retry rule : ",PushRetryPeriod," this time : ",time )
			d := zlib.GetNowTimeSecondToInt() - element.UTime
			//mylog.Info("this time : ",time,"now :",zlib.GetNowTimeSecondToInt() , " - element.UTime ",element.UTime , " = ",d)
			push.Log.Info("this time : ",time,"now :",zlib.GetNowTimeSecondToInt() , " - element.UTime ",element.UTime , " = ",d)
			if d >= time{
				push.pushAndUpInfo(element,PushStatusRetry)
			}else{
				//重试的时间间隔 未满足
				push.Log.Notice("The time is too short to trigger the Push!!! ")
			}
		}
	}
	push.Log.Info("processOne finish")
}
func getAnyType()(a interface{}){
	return a
}

func  (push *Push)  getServiceUri(element PushElement,payload string)(uri string,post interface{},err error){
	postData := getAnyType()
	thirdMethodUri := ""
	success := push.Gamematch.getContainerSuccessByRuleId(push.Rule.Id)
	if element.Category == PushCategorySignTimeout  {
		push.Log.Debug("element.Category == PushCategorySignTimeout")
		postData = GroupStrToStruct(payload)

		thirdMethodUri = "v1/match/error"
	}else if element.Category == PushCategorySuccessTimeout {
		push.Log.Debug("element.Category == PushCategorySuccessTimeout")
		postData = push.QueueSuccess.strToStruct(payload)
		thirdMethodUri = "v1/match/error"
	}else if element.Category == PushCategorySuccess{
		push.Log.Debug("element.Category == PushCategorySuccess")
		thisResult := success.strToStruct (payload)
		//fmt.Printf("%+v", thisResult)
		postData ,_= success.GetResultById(thisResult.Id,1,0)
		thirdMethodUri = "v1/match/succ"
	}else{
		mylog.Error("element.Category error.")
		push.Log.Error("element.Category error.")
		return uri,post,errors.New("element.Category error")
	}
	return thirdMethodUri,postData,nil
}

func  (push *Push)pushAndUpInfo(element PushElement,upStatus int){
	//mylog.Debug("pushAndUpInfo",element,upStatus)
	push.Log.Info("pushAndUpInfo",element," upStatus: ",upStatus)
	var httpRs zlib.ResponseMsgST
	var err error

	payload := strings.Replace(element.Payload,PayloadSeparation,separation,-1)
	thirdMethodUri,postData,err := push.getServiceUri(element,payload)
	if err != nil{
		return
	}
	push.Log.Debug("push third service ,  uri : ",thirdMethodUri, " , postData : ",postData)
	httpRs,err = myservice.HttpPost(SERVICE_MSG_SERVER,thirdMethodUri,postData)
	push.Log.Debug("push third service , httpRs : ",httpRs , " err : ",err)

	if err != nil{
		push.upRetryPushInfo(element)
		msg := myerr.MakeOneStringReplace(err.Error())
		myerr.NewErrorCodeReplace(911,msg)
		push.Log.Error("push third service ",err.Error())
		return
	}

	push.hook(element,httpRs)

}
func  (push *Push)hook(element PushElement,httpRs zlib.ResponseMsgST){
	redisConnFD := myredis.GetNewConnFromPool()
	defer redisConnFD.Close()

	if(httpRs.Code == 0){// 0 即是200 ，推送成功~
		//mymetrics.IncNode("pushSuccess")
		push.delOneByIdInBusiness(redisConnFD,element.Id)
		//push.delOnePush(element.Id)
		//if element.Category == PushCategorySuccess || element.Category == PushCategorySuccessTimeout{
		//	push.Log.Info("delOneResult")
		//	push.QueueSuccess.delOneResult(element.LinkId,1,1,1,1)
		//}
		return
	}

	if element.Category == PushCategorySignTimeout   {
		if httpRs.Code == 116 || httpRs.Code ==  119{
			push.metrics(element.Category,"Ok")
			push.delOnePush(redisConnFD,element.Id)
		}else{
			push.upRetryPushInfo(element)

			httpRsJsonStr,_ := json.Marshal(httpRs)
			msg := myerr.MakeOneStringReplace(string(httpRsJsonStr))
			myerr.NewErrorCodeReplace(700,msg)
			return
		}
	}else if  element.Category == PushCategorySuccessTimeout{
		push.Log.Info("delOneResult")
		push.delOneByIdInBusiness(redisConnFD,element.Id)
		//push.delOnePush(element.Id)
		//push.QueueSuccess.delOneResult(element.LinkId,1,1,1,1)

	}else if element.Category == PushCategorySuccess{
		if httpRs.Code == 108 || httpRs.Code == 102{
			push.upRetryPushInfo(element)

			httpRsJsonStr,_ := json.Marshal(httpRs)
			msg := myerr.MakeOneStringReplace(string(httpRsJsonStr))
			myerr.NewErrorCodeReplace(700,msg)
			return
		}else{
			push.Log.Info("delOneResult")
			push.delOneByIdInBusiness(redisConnFD,element.Id)
			//push.delOnePush(element.Id)
			//push.QueueSuccess.delOneResult(element.LinkId,1,1,1,1)
			return
		}
	}else{
		mylog.Error("pushAndUpInfo element.Category not found!!!")
		push.Log.Error("pushAndUpInfo element.Category not found!!!")
	}
}
func  (push *Push)  delOnePush( redisConn redis.Conn, id int){
	key := push.getRedisKeyPush(id)
	mylog.Info("delOnePush action",id, " key:",key)
	res,err :=   myredis.Send(redisConn,"del",redis.Args{}.Add(key)... )
	//res,err :=   myredis.RedisDo("del",redis.Args{}.Add(key)... )
	mylog.Debug(" delOnePush (",id,")",res,err)
	push.Log.Info(" delOnePush (",id,")",res,err)

	push.delOneStatus(redisConn,id)
}