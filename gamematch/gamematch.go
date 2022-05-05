package gamematch

import (
	"errors"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
	"zlib"
)

type Gamematch struct {
	RuleConfig 			*RuleConfig
	PidFilePath			string
	containerSign 		map[int]*QueueSign	//容器 报名类
	containerSuccess 	map[int]*QueueSuccess
	containerMatch	 	map[int]*Match
	containerPush		map[int]*Push
	//signalChan			map[int] chan int
	signalChan			map[int] map[string] chan int	//一个rule会开N个协作守护协程，之间的通信信号保存在这里
	signalChanRWLock 	*sync.RWMutex
	HttpdRuleState		map[int]int
	PlayerStatus 	*PlayerStatus
	Option 			GamematchOption

}
//玩家结构体，目前暂定就一个ID，其它的字段值，后期得加，主要是用于权重计算
type Player struct {
	Id			int
	MatchAttr	map[string]int
	Weight		float32
}

var mylog 		*zlib.Log
var myerr 		*zlib.Err
var myredis 	*zlib.MyRedis
var myservice	*zlib.Service
var myetcd		*zlib.MyEtcd
var mymetrics 	*zlib.Metrics
var playerStatus 	*PlayerStatus	//玩家状态类
var myGamematchOption *Gamematch
//func (gamematch *Gamematch)countSignalChan(exceptRuleId int)int{
//	cnt :=0
//	for ruleId,set := range gamematch.signalChan{
//		if ruleId == exceptRuleId{
//			continue
//		}
//		cnt += len(set)
//	}
//	return cnt
//}
//

type GamematchOption struct {
	Log 			*zlib.Log
	Redis 			*zlib.MyRedis
	Service 		*zlib.Service
	Etcd 			*zlib.MyEtcd
	Metrics 		*zlib.Metrics
	PidFilePath 	string
	MonitorRuleIds 	[]int
	CmsArg map[string]string
	HttpdOption 	HttpdOption
	//Goroutine		*zlib.Goroutine
}

func NewGamematch(gamematchOption GamematchOption)(gamematch *Gamematch ,errs error){
	//初始化全局变量，用于便捷访问
	mylog = gamematchOption.Log
	myredis = gamematchOption.Redis
	myservice = gamematchOption.Service
	myetcd = gamematchOption.Etcd
	mymetrics = gamematchOption.Metrics
	//初始化 统计信息

	mylog.Info("NewGamematch : ")
	//初始化-错误码
	container := getErrorCode()
	mylog.Info( " init ErrorCodeList , len : ",len(container))
	if   len(container) == 0{
		return gamematch,errors.New("getErrorCode len  = 0")
	}
	//初始化-错误/异常 类
	myerr = zlib.NewErr(mylog,container)
	gamematch = new (Gamematch)
	myGamematchOption = gamematch
	if gamematchOption.PidFilePath == ""{
		return gamematch,myerr.NewErrorCode(501)
	}
	//创建进程ID
	gamematch.PidFilePath = gamematchOption.PidFilePath
	err := gamematch.initPid()
	if err != nil{
		return gamematch,err
	}
	//初始化 - 信号和管道
	gamematch.signalChan =  make( map[int]map[string] chan int )
	gamematch.signalChanRWLock = &sync.RWMutex{}

	//gamematchOption.Goroutine.CreateExec(gamematch,"DemonSignal");

	gamematch.Option = gamematchOption
	//初始化所有 匹配规则
	gamematch.RuleConfig, errs = NewRuleConfig(gamematch,gamematchOption.MonitorRuleIds)
	if errs != nil{
		return gamematch,errs
	}
	//初始化- 容器 : 报名、匹配、推送、报名成功
	gamematch.containerPush		= make( map[int]*Push)
	gamematch.containerSign 	= make(	map[int]*QueueSign )
	gamematch.containerSuccess 	= make(	map[int]*QueueSuccess)
	gamematch.containerMatch	= make( map[int]*Match)
	//实例化容器
	//每一个RULE都有对应的上面的：4个节点
	for _,rule := range gamematch.RuleConfig.getAll(){
		//fmt.Printf("%+v",rule)
		gamematch.containerPush[rule.Id] = NewPush(rule,gamematch)
		gamematch.containerSign[rule.Id] = NewQueueSign(rule,gamematch)
		gamematch.containerSuccess[rule.Id] = NewQueueSuccess(rule,gamematch)
		gamematch.containerMatch[rule.Id] = NewMatch(rule,gamematch)
	}

	containerTotal := len(gamematch.containerPush) + len(gamematch.containerSign) + len(gamematch.containerSuccess) + len(gamematch.containerMatch)
	mylog.Info("rule container total :",containerTotal)
	playerStatus = NewPlayerStatus()
	gamematch.PlayerStatus = playerStatus
	//初始化 - 每个rule - httpd 状态
	gamematch.HttpdRuleState = make(map[int]int)
	mylog.Info("HTTPD_RULE_STATE_INIT")
	for ruleId ,_ := range gamematch.RuleConfig.getAll(){
		//设置状态
		gamematch.HttpdRuleState[ruleId] = HTTPD_RULE_STATE_INIT
	}

	return gamematch,nil
}
//进程PID保存到文件
func (gamematch *Gamematch)initPid()error{
	pid := os.Getpid()
	fd, err  := os.OpenFile(gamematch.PidFilePath, os.O_WRONLY | os.O_CREATE | os.O_TRUNC , 0777)
	defer fd.Close()
	if err != nil{
		rrr := myerr.MakeOneStringReplace(gamematch.PidFilePath + " " + err.Error())
		return myerr.NewErrorCodeReplace(502,rrr)
	}

	_, err = io.WriteString(fd, strconv.Itoa(pid))
	if err != nil{
		rrr := myerr.MakeOneStringReplace(gamematch.PidFilePath + " " + err.Error())
		return myerr.NewErrorCodeReplace(503,rrr)
	}

	mylog.Info("init pid :", pid, " , path : ",gamematch.PidFilePath)
	return nil
}
func (gamematch *Gamematch)GetContainerSignByRuleId(ruleId int)*QueueSign{
	content ,ok := gamematch.containerSign[ruleId]
	if !ok{
		mylog.Error("getContainerSignByRuleId is null")
	}
	return content
}
func (gamematch *Gamematch)getContainerSuccessByRuleId(ruleId int)*QueueSuccess{
	content ,ok := gamematch.containerSuccess[ruleId]
	if !ok{
		mylog.Error("getContainerSuccessByRuleId is null")
	}
	return content
}
func (gamematch *Gamematch)getContainerPushByRuleId(ruleId int)*Push{
	content ,ok := gamematch.containerPush[ruleId]
	if !ok{
		mylog.Error("getContainerPushByRuleId is null")
	}
	return content
}
func (gamematch *Gamematch)getContainerMatchByRuleId(ruleId int)*Match{
	content ,ok := gamematch.containerMatch[ruleId]
	if !ok{
		mylog.Error("getContainerMatchByRuleId is null")
	}
	return content
}
//请便是方便记日志，每次要写两个FD的日志，太麻烦
func rootAndSingToLogInfoMsg(sign *QueueSign,a ...interface{}){
	mylog.Info(a)
	sign.Log.Info(a)
}
//给一个 协程 管道  发信号
func (gamematch *Gamematch)notifyRoutine(sign chan int,signType int){
	mylog.Notice("send routine : ",signType)
	sign <- signType
}
func(gamematch *Gamematch) Quit(source int){
	gamematch.closeDemonRoutine()
	//os.Exit(state)
}
//func(gamematch *Gamematch)Adddd()*Gamematch {
//	return gamematch
//}

//启动后台守护-协程
func (gamematch *Gamematch) Startup(){
	queueList := gamematch.RuleConfig.getAll()
	queueLen := len(queueList)
	mylog.Info("start Startup ,  rule total : ",queueLen)
	if queueLen <=0 {
		mylog.Error(" RuleConfig list is empty!!!")
		zlib.ExitPrint(" RuleConfig list is empty!!!")
		return
	}
	//开始每个rule
	for _,rule := range queueList{
		if rule.Id == 0{//0是特殊管理，仅给HTTPD使用
			continue
		}
		gamematch.startOneRuleDomon(rule)
	}
	//后台守护协程均已开启完毕，可以开启前端HTTPD入口了
	gamematch.StartHttpd(gamematch.Option.HttpdOption)
}

func (gamematch *Gamematch) StartHttpd(httpdOption HttpdOption)error{
	httpd,err  := NewHttpd(httpdOption,gamematch)
	if err != nil{
		return err
	}
	httpd.Start()
	return nil
}
//睡眠 - 协程
func   mySleepSecond(second time.Duration , msg string){
	mylog.Info(msg," sleep second ", strconv.Itoa(int(second)))
	time.Sleep(second * time.Second)
}
//让出当前协程执行时间
func myGosched(msg string){
	mylog.Info(msg + " Gosched ..." )
	runtime.Gosched()
}
//死循环
func deadLoopBlock(sleepSecond time.Duration,msg string){
	for {
		mySleepSecond(sleepSecond,  " deadLoopBlock: " +msg)
	}
}
//实例化，一个LOG类，基于模块
func getModuleLogInc( moduleName string)(newLog *zlib.Log ,err error){
	logOption := myGamematchOption.Option.Log.Option
	logOption.OutFileFileName = moduleName
	//logOption := zlib.LogOption{
	//	OutFilePath : mylog.Op.OutFilePath ,
	//	OutFileName: moduleName + ".log",
	//	//Level : zlib.Atoi(myetcd.GetAppConfByKey("log_level")),
	//	Level:  mylog.Op.Level,
	//	Target : 6,
	//}
	newLog,err = zlib.NewLog(logOption)
	if err != nil{
		return newLog,err
	}
	return newLog,nil
}
//实例化，一个LOG类，基于RULE+模块
func getRuleModuleLogInc(ruleCategory string,moduleName string)*zlib.Log{
	//dir := myetcd.GetAppConfByKey("log_base_path") + "/" + ruleCategory



	logOption := myGamematchOption.Option.Log.Option
	logOption.OutFileFileName = moduleName

	dir := logOption.OutFilePath + "/" + ruleCategory + "/"
	logOption.OutFilePath = dir

	exist ,err := zlib.PathExists(dir)
	if err != nil || exist{//证明目录存在
		//mylog.Debug("dir has exist",dir)
	}else{
		err := os.Mkdir(dir, 0777)
		if err != nil{
			zlib.ExitPrint("create dir failed ",err.Error())
		}else{
			mylog.Debug("create dir success : ",dir)
		}
	}

	//logOption := zlib.LogOption{
	//	OutFilePath : dir ,
	//	OutFileName: moduleName + ".log",
	//	Level : mylog.Op.Level,
	//	Target : 6,
	//}
	newLog,err := zlib.NewLog(logOption)
	if err != nil{
		zlib.ExitPrint(err.Error())
	}
	return newLog
}
func (gamematch *Gamematch) DelOneRuleById(ruleId int){
	gamematch.RuleConfig.delOne(ruleId)
}

//删除全部数据
func (gamematch *Gamematch) DelAll(){
	mylog.Notice(" action :  DelAll")
	keys := redisPrefix + "*"
	myredis.RedisDelAllByPrefix(keys)
}
//开启一条rule的所有守护协程，
//虽然有4个，但是只有match是最核心、最复杂的，另外3个算是辅助
func  (gamematch *Gamematch)startOneRuleDomon(rule Rule){
	//zlib.AddRoutineList("startOneRuleDomon push")
	//zlib.AddRoutineList("startOneRuleDomon signTimeout")
	//zlib.AddRoutineList("startOneRuleDomon successTimeout")
	//zlib.AddRoutineList("startOneRuleDomon matching")

	//报名超时,这里注释掉了，因为：是并发协程开启，如果其它协程先执行检测到有要处理的数据，这个时候此协程检查出有超时的数据开始删除操作，结果其它协程执行到一半发现数据丢了
	//queueSign := gamematch.GetContainerSignByRuleId(rule.Id)
	//go gamematch.StartOneGoroutineDemon(rule.Id,"signTimeout",queueSign.Log,queueSign.CheckTimeout)
	//gamematch.Option.Goroutine.CreateExec(gamematch,"MyDemon",rule.Id,"signTimeout",queueSign.Log,queueSign.CheckTimeout)
	//报名成功，但3方迟迟推送失败，无人接收，超时
	queueSuccess := gamematch.getContainerSuccessByRuleId(rule.Id)
	go gamematch.StartOneGoroutineDemon(rule.Id,"successTimeout",queueSuccess.Log,queueSuccess.CheckTimeout)
	//gamematch.Option.Goroutine.CreateExec(gamematch,"MyDemon",rule.Id,"successTimeout",queueSuccess.Log,queueSuccess.CheckTimeout)
	//推送
	push := gamematch.getContainerPushByRuleId(rule.Id)
	go gamematch.StartOneGoroutineDemon(rule.Id,"push",push.Log,push.checkStatus)
	//gamematch.Option.Goroutine.CreateExec(gamematch,"MyDemon",rule.Id,"push",push.Log,push.checkStatus)
	//匹配
	match := gamematch.containerMatch[rule.Id]
	go gamematch.StartOneGoroutineDemon(rule.Id,"matching",match.Log,match.matching)
	//gamematch.Option.Goroutine.CreateExec(gamematch,"MyDemon",rule.Id,"matching",match.Log,match.matching)

	mylog.Info("start httpd ,up state...")
	gamematch.HttpdRuleState[rule.Id] = HTTPD_RULE_STATE_OK
}

