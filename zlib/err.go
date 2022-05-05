package zlib

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)
const(
	CODE_NOT_EXIST = 555
)
type ErrInfo struct {
	Code  int
	Msg   string
	Where string
	Flag  string
	JsonMsg string
}

func (errInfo *ErrInfo) Error()string{
	return errInfo.JsonMsg
}

type Err struct{
	container map[int]ErrInfo
	log 	*Log
}
//构造函数
func NewErr(log *Log,container []string)*Err{
	err := new(Err)
	err.container = make( map[int]ErrInfo )
	for _,v:= range container{
		row := strings.Split(v,",")
		err.setOneContainerElement(Atoi(row[0]),row[1],row[2])
	}
	//定义一个公共错误码，当用户传入一个不存在 的CODE里，报错
	errInfo :=ErrInfo {
		Code: CODE_NOT_EXIST,
		Msg: "code not found",
		Flag: "err",
	}

	err.container[CODE_NOT_EXIST] = errInfo
	err.log = log

	return err
}

func  (e *Err)setOneContainerElement(code int ,msg string,flag string){
	errInfo :=ErrInfo {
		Code: code,
		Msg: msg,
		Flag:flag,
	}

	e.container[code] = errInfo
}
// 声明一个错误
func  (e *Err)NewErrorCode(code int) error {
	where := e.caller(1, false)
	errInfo,ok :=  e.container[code]
	if !ok{
		return e.returnErrorInfo(e.container[CODE_NOT_EXIST])
	}
	errInfo.Where = where
	return e.returnErrorInfo(errInfo)

}

// 声明一个错误
func  (e *Err)NewError(code int, msg string) error {
	where := e.caller(1, false)
	errInfo,ok :=  e.container[code]
	if !ok{
		return e.returnErrorInfo(e.container[CODE_NOT_EXIST])
	}
	errInfo.Where = where
	if msg != ""{
		errInfo.Msg = msg
	}
	return e.returnErrorInfo(errInfo)

}

func  (e *Err)NewErrorCodeReplace(code int,replace map[int]string) error {
	where := e.caller(1, false)
	errInfo,ok :=  e.container[code]
	if !ok{
		return e.returnErrorInfo(e.container[CODE_NOT_EXIST])
	}
	errInfo.Where = where
	for k,v :=  range replace{
		errInfo.Msg = strings.Replace(errInfo.Msg,"{"+strconv.Itoa(k)+"}",v,-1)
	}
	return e.returnErrorInfo(errInfo)
}
func  (e *Err) returnErrorInfo(errInfo ErrInfo)*ErrInfo{
	e.log.Error(errInfo)

	errorJsonStr,_ := json.Marshal(errInfo)
	errInfo.JsonMsg = string(errorJsonStr)
	return &errInfo
}
// 对一个错误追加信息
func  (e *Err)Wrap(err error, extMsg ...string) *ErrorCode {
	msg  := err.Error()
	if len(extMsg) != 0 {
		msg = strings.Join(extMsg, " : ") + " : " + msg
	}
	return &ErrorCode{Msg: msg}
}

// 获取源代码行数
func  (e *Err)caller(calldepth int, short bool) string {
	_, file, line, ok := runtime.Caller(calldepth + 1)
	if !ok {
		file = "???"
		line = 0
	} else if short {
		file = filepath.Base(file)
	}

	return fmt.Sprintf("%s:%d", file, line)
}

func  (e *Err)MakeOneStringReplace(str string)map[int]string{
	msg := make(map[int]string)
	msg[0] = str
	return msg
}



