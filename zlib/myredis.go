package zlib

import (
	"errors"
	"github.com/gomodule/redigo/redis"
	"time"
)

type MyRedis struct {
	option RedisOption
	Conn redis.Conn
	connPool *redis.Pool
}


type RedisOption struct {
	Host string
	Port string
	Ps	string
	Log *Log
}

func   NewRedisConn(redisOption RedisOption)(*MyRedis,error){
	myRedis := new(MyRedis)
	redisOption.Log.Info("NewRedisConn : ",redisOption.Host,redisOption.Port)
	conn,error := redis.Dial("tcp",redisOption.Host+":"+redisOption.Port)
	if error != nil{
		return nil, error
	}
	myRedis.option = redisOption
	myRedis.Conn = conn
	return myRedis,nil
}

func NewRedisConnPool(redisOption RedisOption)(*MyRedis,error){
	myRedis := new(MyRedis)
	redisOption.Log.Info("NewRedisConn ,host : ",redisOption.Host, " port : " ,redisOption.Port , " ps:",redisOption.Ps)
	myRedisPool  := &redis.Pool{
		// 从配置文件获取maxidle以及maxactive，取不到则用后面的默认值
		MaxIdle:     180,
		MaxActive:   200,
		IdleTimeout: 180 * time.Second,
		Dial: func() (redis.Conn, error) {
			var err error
			var c redis.Conn
			if redisOption.Ps != ""{
				c, err = redis.Dial("tcp", redisOption.Host+":"+redisOption.Port,redis.DialPassword(redisOption.Ps))
			}else{
				c, err = redis.Dial("tcp", redisOption.Host+":"+redisOption.Port)
			}

			if err != nil {
				return nil, err
			}
			// 选择db
			c.Do("SELECT", 1)
			return c, nil
		},
		Wait: true,//如果获取不到，即阻塞
	}
	myRedis.option = redisOption
	myRedis.connPool = myRedisPool
	redisOption.Log.Info("test redis conn fd : ping ")

	if redisOption.Ps != ""{
		_,err :=myRedis.RedisDo("AUTH",redisOption.Ps)
		if err != nil{
			return nil,errors.New("redis AUTH err:"+redisOption.Ps)
		}
	}

	_,err :=myRedis.RedisDo("ping")
	return myRedis,err
}
func  (myRedis *MyRedis)Shutdown(){
	//myRedis.connPool.Close()
	myRedis.option.Log.Alert("redis shutdown.")
}
func  (myRedis *MyRedis)GetNewConnFromPool()redis.Conn{
	//myRedis.option.Log.Debug("redis :get new conn FD from pool.")
	conn := myRedis.connPool.Get()
	return conn
}
//指定一个 sock fd
func  (myRedis *MyRedis)ConnDo(conn redis.Conn,commandName string, args ...interface{})(reply interface{}, error error){
	//myRedis.option.Log.Debug("[redis]connDo  :",commandName,args)
	res,error :=conn.Do(commandName,args...)
	if error != nil{
		myRedis.option.Log.Error("redis ConnDo err :",error.Error())
		return nil, error
	}
	return res,error
}
func  (myRedis *MyRedis)Exec(conn redis.Conn)(reply interface{}, error error){
	rs,err := myRedis.ConnDo(conn,"exec")
	//myRedis.option.Log.Info("redis : exec , rs : ",rs,"err:",err)
	if err != nil{
		myRedis.option.Log.Error("transaction failed : ",err)
	}
	return rs,err
}
func  (myRedis *MyRedis)Multi(conn redis.Conn)(reply interface{}, error error){
	//myRedis.option.Log.Debug("[redis]Multi  ")
	return myRedis.Send(conn,"Multi")
}

func  (myRedis *MyRedis)Send(conn redis.Conn,commandName string, args ...interface{})(reply interface{}, error error){
	err := conn.Send(commandName,args...)
	myRedis.option.Log.Debug("[redis]Send : ",commandName , args," err : ",err)
	return reply,err
}

//func  (myRedis *MyRedis)Exec(conn redis.Conn)(reply interface{}, error error){
//	myRedis.option.Log.Debug("[redis]Exec  ")
//	return myRedis.ConnDo(conn,"EXEC")
//}


func  (myRedis *MyRedis)RedisDo(commandName string, args ...interface{})(reply interface{}, error error){
	//myRedis.option.Log.Debug("[redis]redisDo init:",commandName,args)
	conn := myRedis.GetNewConnFromPool()
	defer conn.Close()
	res,error := conn.Do(commandName,args... )

	//res,error := myRedis.Conn.Do(commandName,args... )
	if error != nil{
		myRedis.option.Log.Notice("redis err :",error.Error())
		return nil, error
	}
	//reflect.ValueOf(res).IsNil(),reflect.ValueOf(res).Kind(),reflect.TypeOf(res)
	//zlib.MyPrint("redisDo exec ,res : ",res," err :",err)
	return res,error
}

func  (myRedis *MyRedis)RedisDelAllByPrefix(prefix string){
	myRedis.option.Log.Notice(" action redisDelAllByPrefix : ",prefix)
	res,err := redis.Strings(  myRedis.RedisDo("keys",prefix))
	if err != nil{
		ExitPrint("redis keys err :",err.Error())
	}
	myRedis.option.Log.Debug("del element will num :",len(res))
	if len(res) <= 0 {
		myRedis.option.Log.Notice(" keys is null,no need del...")
		return
	}
	for _,v := range res{
		res,_ := myRedis.RedisDo("del",v)
		myRedis.option.Log.Debug("del key ",v , " ,  rs : ",res)
	}
}

func  (myRedis *MyRedis) redisDelAll(redisPrefix string){
	myRedis.RedisDelAllByPrefix( redisPrefix)
}