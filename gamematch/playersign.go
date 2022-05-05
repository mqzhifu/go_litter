package gamematch

import (
	"regexp"
	"strconv"
	"strings"
	"zlib"
)

//报名 - 加入匹配队列
//此方法，得有前置条件：验证所有参数是否正确，因为使用者为http请求，数据的验证交由HTTP层做处理，如果是非HTTP，要验证一下
func (gamematch *Gamematch) Sign(httpReqBusiness HttpReqBusiness )( group Group , err error){
	ruleId := httpReqBusiness.RuleId
	outGroupId :=  httpReqBusiness.GroupId
	//这里只做最基础的验证，前置条件是已经在HTTP层作了验证
	rule,ok := gamematch.RuleConfig.GetById(ruleId)
	if !ok{
		return group,myerr.NewErrorCode(400)
	}
	lenPlayers := len(httpReqBusiness.PlayerList)
	if lenPlayers == 0{
		return group,myerr.NewErrorCode(401)
	}
	//groupsTotal := queueSign.getAllGroupsWeightCnt()	//报名 小组总数
	//playersTotal := queueSign.getAllPlayersCnt()	//报名 玩家总数
	//mylog.Info(" action :  Sign , players : " + strconv.Itoa(lenPlayers) +" ,queue cnt : groupsTotal",groupsTotal ," , playersTotal",playersTotal)
	queueSign := gamematch.GetContainerSignByRuleId(ruleId)
	rootAndSingToLogInfoMsg(queueSign,"new sign :[ruleId : ",ruleId,"(",rule.CategoryKey,") , outGroupId : ",outGroupId," , playersCount : ",lenPlayers,"] ")
	//mylog.Info("new sign :[ruleId : ",ruleId,"(",rule.CategoryKey,") , outGroupId : ",outGroupId," , playersCount : ",lenPlayers,"] ")
	//queueSign.Log.Info("new sign :[ruleId : " ,  ruleId   ,"(",rule.CategoryKey,") , outGroupId : ",outGroupId," , playersCount : ",lenPlayers,"] ")
	now := zlib.GetNowTimeSecondToInt()

	//zlib.PrintStruct(queueSign.Rule,":")
	if lenPlayers > queueSign.Rule.GroupPersonMax{
		msg := make(map[int]string)
		msg[0] = strconv.Itoa( queueSign.Rule.GroupPersonMax)
		return group,myerr.NewErrorCodeReplace(408,msg)
	}
	queueSign.Log.Info("start check player status :")
	//检查，所有玩家的状态
	var  players []Player
	for _,httpPlayer := range httpReqBusiness.PlayerList{
		player := Player{Id: httpPlayer.Uid,MatchAttr:httpPlayer.MatchAttr}
		playerStatusElement,isEmpty := playerStatus.GetById(player.Id)
		queueSign.Log.Info("player(",player.Id , ") GetById :  status = ", playerStatusElement.Status, " isEmpty:",isEmpty)
		if isEmpty == 1{
			//这是正常
		}else if playerStatusElement.Status == PlayerStatusSuccess{//玩家已经匹配成功，并等待开始游戏
			queueSign.Log.Error(" player status = PlayerStatusSuccess ,demon not clean.")
			msg := make(map[int]string)
			msg[0] = strconv.Itoa(player.Id)
			return group,myerr.NewErrorCodeReplace(403,msg)
		}else if playerStatusElement.Status == PlayerStatusSign{//报名成功，等待匹配
			isTimeout := playerStatus.checkSignTimeout(rule,playerStatusElement)
			if !isTimeout{//未超时
				//queueSign.Log.Error(" player status = matching...  not timeout")
				msg := make(map[int]string)
				msg[0] = strconv.Itoa(player.Id)
				return group,myerr.NewErrorCodeReplace(402,msg)
			}else{//报名已超时，等待后台守护协程处理
				//这里其实也可以先一步处理，但是怕与后台协程冲突
				//queueSign.Log.Error(" player status is timeout ,but not clear , wait a moment!!!")
				msg := make(map[int]string)
				msg[0] = strconv.Itoa(player.Id)
				return group,myerr.NewErrorCodeReplace(407,msg)
			}
		}
		players  = append(players,player)
		//playerStatusElementMap[player.Id] = playerStatusElement
	}
	rootAndSingToLogInfoMsg(queueSign,"finish check player status.")
	//验证3方传过来的groupId 是否重复
	allGroupIds := queueSign.GetGroupSignTimeoutAll()
	for _,hasGroupId := range allGroupIds{
		if outGroupId == hasGroupId{
			zlib.MyPrint(allGroupIds,outGroupId,hasGroupId,httpReqBusiness)
			msg := make(map[int]string)
			msg[0] = strconv.Itoa(outGroupId)
			return group,myerr.NewErrorCodeReplace(409,msg)
		}
	}
	//zlib.ExitPrint(allGroupIds)
	//先计算一下权重平均值
	var groupWeightTotal float32
	groupWeightTotal = 0.00

	if rule.PlayerWeight.Formula != ""  {
		rootAndSingToLogInfoMsg(queueSign,"rule weight , Formula : ",rule.PlayerWeight.Formula )
		var weight float32
		weight = 0.00
		var playerWeightValue []float32
		for k,p := range players{
			onePlayerWeight := getPlayerWeightByFormula(rule.PlayerWeight.Formula ,p.MatchAttr ,queueSign)
			mylog.Debug("onePlayerWeight : ",onePlayerWeight)
			if onePlayerWeight > WeightMaxValue {
				onePlayerWeight = WeightMaxValue
			}
			weight += onePlayerWeight
			playerWeightValue = append(playerWeightValue,onePlayerWeight)
			players[k].Weight = onePlayerWeight
		}
		switch rule.PlayerWeight.Aggregation{
		case "sum":
			groupWeightTotal = weight
		case "min":
			groupWeightTotal = zlib.FindMinNumInArrFloat32(playerWeightValue)
		case "max":
			groupWeightTotal = zlib.FindMaxNumInArrFloat32(playerWeightValue)
		case "average":
			groupWeightTotal =  weight / float32(len(players))
		default:
			groupWeightTotal =  weight / float32(len(players))
		}
		//保留2位小数
		tmp := zlib.FloatToString(groupWeightTotal,2)
		groupWeightTotal = zlib.StringToFloat(tmp)
	}else{
		rootAndSingToLogInfoMsg(queueSign,"rule weight , Formula is empty!!!")
	}
	//下面两行必须是原子操作，如果pushOne执行成功，但是upInfo没成功会导致报名队列里，同一个用户能再报名一次
	redisConnFD := myredis.GetNewConnFromPool()
	defer redisConnFD.Close()
	//开始多指令缓存模式
	myredis.Multi(redisConnFD)

	//超时时间
	expire := now + rule.MatchTimeout
	//创建一个新的小组
	group =  gamematch.NewGroupStruct(rule)
	//这里有偷个懒，还是用外部的groupId , 不想再给redis加 groupId映射outGroupId了
	mylog.Notice(" outGroupId replace groupId :",outGroupId,group.Id)
	group.Id = outGroupId
	group.Players = players
	group.SignTimeout = expire
	group.Person = len(players)
	group.Weight = groupWeightTotal
	group.OutGroupId = outGroupId
	group.Addition = httpReqBusiness.Addition
	group.CustomProp =  httpReqBusiness.CustomProp
	group.MatchCode = rule.CategoryKey
	rootAndSingToLogInfoMsg(queueSign,"newGroupId : ",group.Id , "player/group weight : " ,groupWeightTotal ," now : ",now ," expire : ",expire)
	//mylog.Info("newGroupId : ",group.Id , "player/group weight : " ,groupWeightTotal ," now : ",now ," expire : ",expire )
	//queueSign.Log.Info("newGroupId : ",group.Id , "player/group weight : " ,groupWeightTotal ," now : ",now ," expire : ",expire)
	queueSign.AddOne(group,redisConnFD)
	playerIds := ""
	for _,player := range players{

		newPlayerStatusElement := playerStatus.newPlayerStatusElement()
		newPlayerStatusElement.PlayerId = player.Id
		newPlayerStatusElement.Status = PlayerStatusSign
		newPlayerStatusElement.RuleId = ruleId
		newPlayerStatusElement.Weight = player.Weight
		newPlayerStatusElement.SignTimeout = expire
		newPlayerStatusElement.GroupId = group.Id

		queueSign.Log.Info("playerStatus.upInfo:" ,PlayerStatusSign)
		playerStatus.upInfo(  newPlayerStatusElement , redisConnFD)

		playerIds += strconv.Itoa(player.Id) + ","
	}
	//提交缓存中的指令
	_,err = myredis.Exec(redisConnFD )
	if err != nil{
		queueSign.Log.Error("transaction failed : ",err)
	}
	queueSign.Log.Info(" sign finish ,total : newGroupId " , group.Id ," success players : ",len(players))
	mylog.Info(" sign finish ,total : newGroupId " , group.Id ," success players : ",len(players))

	//signSuccessReturnData = SignSuccessReturnData{
	//	RuleId: ruleId,
	//	GroupId: outGroupId,
	//	PlayerIds: playerIds,
	//
	//}

	return group,nil
}

func getPlayerWeightByFormula(formula string,MatchAttr map[string]int,sign *QueueSign)float32{
	//mylog.Debug("getPlayerWeightByFormula , formula:",formula)
	grep := FormulaFirst + "([\\s\\S]*?)"+FormulaEnd
	var imgRE = regexp.MustCompile(grep)
	findRs := imgRE.FindAllStringSubmatch(formula, -1)
	rootAndSingToLogInfoMsg(sign,"parse PlayerWeightByFormula : ",findRs)
	if len(findRs) == 0{
		return 0
	}
	for _,v := range findRs {
		val ,ok := MatchAttr[v[1]]
		if !ok {
			val = 0
		}
		formula = strings.Replace(formula,v[0],strconv.Itoa(val),-1)

	}
	rootAndSingToLogInfoMsg(sign,"final formula replaced str :",formula)
	rs, err := zlib.Eval(formula)
	if err != nil{
		return 0
	}
	f,_ :=rs.Float32()
	return f
}
