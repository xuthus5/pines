package main

import (
	"encoding/json"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"strconv"
)

// Write 输出返回结果
func Write(w http.ResponseWriter, response []byte) {
	//公共的响应头设置
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, OPTIONS")
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(string(response))))
	_, _ = w.Write(response)
	return
}

// Ping 检测服务连通性
func Ping(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	response, _ := json.Marshal(struct {
		Ping string `json:"ping"`
	}{
		Ping: "PONG",
	})
	Write(w, response)
}

// TokenAuth 检测Token合法性
func TokenAuth(token string) bool {
	if token != config.Token {
		return false
	}
	return true
}

// Login 登录
func Login(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if !TokenAuth(r.URL.Query().Get("token")) {
		response, _ := json.Marshal(struct {
			Code   int    `json:"code"`
			Errors string `json:"errors"`
		}{
			Code:   500,
			Errors: "token error",
		})
		Write(w, response)
	}
	response, _ := json.Marshal(struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data"`
	}{
		Code:    200,
		Message: "ok",
		Data:    config.Token,
	})
	Write(w, response)
}

// GetUploadAPI 获取快捷上传接口
func GetUploadAPI(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	response, _ := json.Marshal(struct {
		Code   int         `json:"code"`
		Utoken string      `json:"utoken"`
		Url    interface{} `json:"url"`
	}{
		Code:   200,
		Utoken: config.UToken,
		Url:    config.Default,
	})
	Write(w, response)
}

// API 请求处理
func API(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// 检测token 为空或验证不通过
	if r.Header.Get("utoken") == config.UToken {
		goto UNCHECK
	}
	if r.Header.Get("token") == "" || !TokenAuth(r.Header.Get("token")) {
		response, _ := json.Marshal(struct {
			Errors string `json:"errors"`
		}{
			Errors: "token error",
		})
		Write(w, response)
		return
	}
UNCHECK:
	var result []byte
	if ps.ByName("type") == "cos" {
		result = CosHandler(r)
	} else if ps.ByName("type") == "oss" {
		result = OssHandler(r)
	} else if ps.ByName("type") == "ups" {
		result = UpsHandler(r)
	}
	Write(w, result)
}

func Router() *httprouter.Router {
	router := httprouter.New()
	router.GlobalOPTIONS = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		header := w.Header()
		header.Set("Access-Control-Allow-Origin", "*")
		header.Set("Access-Control-Allow-Headers", "*")
		header.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, OPTIONS")
		// Adjust status code to 204
		w.WriteHeader(http.StatusNoContent)
	})
	router.GET("/ping", Ping)
	router.GET("/login", Login)
	router.GET("/get_upload_config", GetUploadAPI)
	router.GET("/api/:type", API)
	router.POST("/api/:type", API)
	router.NotFound = http.FileServer(http.Dir("dist"))
	return router
}
