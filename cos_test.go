package goupload_cos_test

import (
	"context"
	"encoding/json"
	logger "github.com/kordar/gologger"
	"github.com/kordar/goupload_cos"
	"testing"
)

var client = goupload_cos.NewCOSClient("", "", "", "")
var targetDir = "/Users/mac/Pictures/bucket"

func TestCosUploader_Get(t *testing.T) {
	bytes, err := client.Get(context.Background(), "333.txt")
	logger.Error("--------------", err)
	logger.Infof("==============%v", string(bytes))
}

func TestCosUploader_GetToFile(t *testing.T) {
	err := client.GetToFile(context.Background(), "333.txt", targetDir+"/ww.txt")
	logger.Error("--------------", err)
}

func TestCosUploader_Put(t *testing.T) {
	err := client.PutString(context.Background(), "444.txt", "AAAAA")
	logger.Error("--------------", err)
}

func TestCosUploader_PutFromFile(t *testing.T) {
	err := client.PutFromFile(context.Background(), "666.jpg", targetDir+"/111233.jpg")
	logger.Error("--------------", err)
}

func TestCosUploader_Del(t *testing.T) {
	err := client.Del(context.Background(), "333.txt")
	logger.Error("--------------", err)
}

func TestCosUploader_DelAll(t *testing.T) {
	client.DelAll(context.Background(), "AA")
}

func TestCosUploader_Count(t *testing.T) {
	count := client.Count(context.Background(), "images/")
	logger.Info("--------------", count)
}

func TestCosUploader_List(t *testing.T) {
	list, next := client.List(context.Background(), "", "", 2, false)
	marshal, _ := json.Marshal(list)
	logger.Info(string(marshal))
	logger.Infof("--------------%v", next)
}

func TestCosUploader_AppendString(t *testing.T) {
	pos, err := client.AppendString(context.Background(), "666.txt", 7, "AAAAAA\n")
	logger.Error("--------------", err)
	logger.Info("---------------", pos)
}

func TestCosUploader_IsExist(t *testing.T) {
	exist, err := client.IsExist(context.Background(), "images/demo/")
	logger.Info("---------------", exist, err)
}

func TestCosUploader_Copy(t *testing.T) {
	err := client.Copy(context.Background(), "demo22/", "images/demo/")
	logger.Error("--------------", err)
}

func TestCosUploader_Move(t *testing.T) {
	err := client.Move(context.Background(), "123.txt", "444.txt")
	logger.Error("--------------", err)
}

func TestCosUploader_Rename(t *testing.T) {
	err := client.Rename(context.Background(), "1233.txt", "123.txt")
	logger.Error("--------------", err)
}

func TestCosUploader_Tree(t *testing.T) {
	list := client.Tree(context.Background(), "", "", 1000, 0, 1, false, true)
	marshal, _ := json.Marshal(list)
	logger.Info(string(marshal))
}
