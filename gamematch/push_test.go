package gamematch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"zlib"
)
func TestHttp(t *testing.T) {
	zlib.LogLevelFlag = zlib.LOG_LEVEL_DEBUG

	fmt.Println("http demon start:...")
	http.HandleFunc("/", wwwHandle)
	err := http.ListenAndServe(TEST_HTTP_PUSH_RECEIVE_HOST, nil)
	if err != nil {
		fmt.Println("ListenAndServe err:",err)
	}
}
var success = make(map[string]int)
func wwwHandle(w http.ResponseWriter, r *http.Request) {
	uri := r.URL.RequestURI()
	body := make([]byte, r.ContentLength)
	r.Body.Read(body)

	fmt.Println("wwwHandle : uri ",uri , " method ",r.Method  )


	if uri == "/v1/match/succ"{
		result := Result{}
		errs := json.Unmarshal(body,&result)
		if errs != nil{
			fmt.Println("json.Unmarshal:",errs)
		}else{
			//Id 			int
			//RuleId		int
			//MatchCode	string
			//ATime		int			//匹配成功的时间
			//Timeout		int			//多少秒后无人来取，即超时，更新用户状态，删除数据
			//Teams		[]int		//该结果，有几个 队伍，因为每个队伍是一个集合，要用来索引
			//PlayerIds	[]int
			//GroupIds	[]int
			//PushId		int
			//Groups		[]Group		//该结果下包含的两个组详细信息，属性挂载，用于push payload
			//PushElement	PushElement	//该结果下推送的详细信息，属性挂载

			//zlib.PrintStruct(result,":")
			zlib.MyPrint("groupIds:",result.GroupIds , " len:",len(result.GroupIds))
			success["group"] = success["group"] + len(result.Groups)
			for _,group:= range result.Groups{
				success["players"] = success["players"] + len(group.Players)
				playerIds := ""
				for _,player := range group.Players{
					playerIds += strconv.Itoa(player.Id) + ", "
				}
				zlib.MyPrint("group playerIds:  ",playerIds)

			}

			zlib.MyPrint("total:",success)

		}

	}else if uri == "/v1/match/error"{
		//fmt.Println("match error:",string(body))
		group := Group{}
		errs := json.Unmarshal(body,&group)
		if errs != nil{
			fmt.Println("/v1/match/error json.Unmarshal:",errs)
		}else{
			playerIds := ""
			for _,player := range group.Players{
				playerIds += strconv.Itoa(player.Id) + ", "
			}

			fmt.Println("groupId : ",group.Id," playerIds : ",playerIds)
		}
	}else{
		fmt.Println("uri err,not match.")
	}

	responseMsgST := ResponseMsgST{
		Code: 0,
	}
	str ,_ := json.Marshal(responseMsgST)
	w.Write(str)
}