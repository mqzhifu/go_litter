package zlib

import (
	"io"
	"os"
	"regexp"
	"strings"
)

type DocRegular struct {
	FileDir string
	content string
}

func NewDocRegular(fileDir string)*DocRegular{
	fd ,err := os.OpenFile(fileDir,os.O_RDONLY,0666)

	if err != nil{
		ExitPrint(err)
	}

	defer fd.Close()
	var strings string
	for {
		buf := make([]byte, 1024)
		n, err := fd.Read(buf)
		if err != nil && err != io.EOF {
			panic(err)
		}
		if n == 0 {
			break
		}
		strings += string(buf)
	}

	docRegular := new(DocRegular)
	docRegular.content = strings

	return docRegular

}

func (docRegular *DocRegular)ParseConst(){
	//"const([\s\S]*)"
	//reg := regexp.MustCompile(`//(?s).*const(?s).*"`)
	reg := regexp.MustCompile(`//([\s\S]*?)const([\s\S]*?)\)+`)
	find := reg.FindAllString(docRegular.content, -1)
	constlist := make(map[string][]map[string]string)
	for _,v := range find{
		list := strings.Split(v,"\n")
		blockDesc := list[0]
		contentArr := list[2:len(list)-1]
		contentMap := make(map[string]string)
		var container []map[string]string
		for _,v2 := range contentArr{
			if v2 == "" || v2 == "\n" || strings.Contains(v2, "=") == false {
				continue
			}
			tmp := strings.Split(v2,"=")
			key := strings.TrimSpace(tmp[0])
			valDesc := strings.TrimSpace(tmp[1])
			valDescArr := strings.Split(valDesc,"//")
			if len(valDescArr )== 2{
				contentMap["val"] = valDescArr[0]
				contentMap["desc"] = valDescArr[1]
			}else{
				contentMap["val"] = valDescArr[0]
				contentMap["desc"] = ""
			}
			contentMap[	"key"] = key
			container  = append(container,contentMap)
		}
		constlist[blockDesc] = container

	}
	ExitPrint(constlist)

}