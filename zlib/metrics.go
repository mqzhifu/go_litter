package zlib

import (
	"context"
	"sync"
)

const(
	METRICS_OPT_PLUS 	= 1  	//1累加
	METRICS_OPT_INC 	= 2		//2加加
	METRICS_OPT_LESS 	= 3		//3累减
	METRICS_OPT_DIM 	= 4		//4减减
)

type MetricsChanMsg struct {
	Key 	string
	Value 	int
	Opt 	int	//1累加2加加3累减4减减
}

type Metrics struct {
	//Container map[string]int
	Option MetricsOption
	Input 		chan MetricsChanMsg
	Close 		chan int
	Pool 		map[string]int
	PoolRWLock *sync.RWMutex
	//ReadPool 	map[string]int	//防止并发IO-map造成fatal，复制pool变量，专供读操作
	//PoolHasChange bool
}

type MetricsOption struct {
	Log *Log
}

func NewMetrics(op MetricsOption)*Metrics{
	metrics := new (Metrics)
	//metrics.Container = make(map[string]int)
	metrics.Option = op
	metrics.Input = make(chan MetricsChanMsg)
	metrics.Close = make(chan int)
	metrics.Pool =  make(map[string]int)
	metrics.PoolRWLock = &sync.RWMutex{}
	return metrics
}

//func (metrics *Metrics) CreateOneNode(name string){
//	metrics.Container[name] = 0
//}
//
func (metrics *Metrics) GetOne(name string)int{
	metrics.PoolRWLock.RLock()
	defer metrics.PoolRWLock.RUnlock()
	return metrics.Pool[name]
}

func (metrics *Metrics) GetAll()map[string]int{
	metrics.PoolRWLock.RLock()
 	defer metrics.PoolRWLock.RUnlock()
	pool := make(map[string]int)
	if len(metrics.Pool)<=0{
		return pool
	}

	for k,v := range metrics.Pool{
		pool[k] = v
	}
	return pool
}

//程序执行时间
func (metrics *Metrics) GetExecTime( )int{
	now := GetNowTimeSecondToInt()
	return now - metrics.GetOne("StartUpTime")
}
//程序初始化时间
func (metrics *Metrics) GetInitTime( )int{
	return metrics.GetOne("InitEndTime") - metrics.GetOne("StartUpTime")
}


func  (metrics *Metrics)Start(ctx context.Context){
	//defer func(ctx context.Context ) {
	//	if err := recover(); err != nil {
	//		myNetWay.RecoverGoRoutine(metrics.start,ctx,err)
	//	}
	//}(ctx)

	metrics.Option.Log.Alert("metrics start:")
	ctxHasDone := 0
	for{
		select {
		case metricsChanMsg := <- metrics.Input:
			metrics.processMsg(metricsChanMsg)
		case <- metrics.Close:
			ctxHasDone = 1
		}
		if ctxHasDone == 1{
			goto end
		}
	}
end:
	metrics.Option.Log.Alert(" zlib.metrics.")
}

func  (metrics *Metrics)FastLog(key string,opt int ,value int ){
	metricsChanMsg := MetricsChanMsg{
		Key :key,
		Opt: opt,
		Value: value,
	}
	metrics.Input <- metricsChanMsg
}
func (metrics *Metrics)Shutdown(){
	metrics.Option.Log.Alert("shutdown metrics")
	metrics.Close <- 1
}

func (metrics *Metrics)processMsg(metricsChanMsg MetricsChanMsg){
	metrics.PoolRWLock.Lock()
	defer metrics.PoolRWLock.Unlock()

	if metricsChanMsg.Opt == METRICS_OPT_PLUS{
		metrics.Pool[metricsChanMsg.Key] += metricsChanMsg.Value
	}else if metricsChanMsg.Opt == METRICS_OPT_INC {
		metrics.Pool[metricsChanMsg.Key]++
	}else if metricsChanMsg.Opt == METRICS_OPT_LESS{
		metrics.Pool[metricsChanMsg.Key] -= metricsChanMsg.Value
	}else if metricsChanMsg.Opt == METRICS_OPT_DIM{
		metrics.Pool[metricsChanMsg.Key]--
	}else{
		MyPrint("processMsg opt err",metricsChanMsg)
	}
}


//
//func (metrics *Metrics)PlusNode(name string ,num int){
//	metrics.Container[name] = metrics.Container[name] + num
//}
//
//func (metrics *Metrics)SetNode(name string ,num int){
//	metrics.Container[name] = num
//}
//
//func (metrics *Metrics)IncNode(name string  ){
//	metrics.Container[name] ++
//}
//
//func (metrics *Metrics)LessNode(name string  ){
//	metrics.Container[name] --
//}
//
//
//func (metrics *Metrics)GetAllAndClear()map[string]int{
//	container := metrics.Container
//	metrics.Container = make(map[string]int)
//	return  container
//}
//
//func (metrics *Metrics)GetAll()map[string]int{
//	container := metrics.Container
//	return  container
//}