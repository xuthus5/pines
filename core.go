package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/tencentyun/cos-go-sdk-v5"
	"github.com/upyun/go-sdk/upyun"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config 配置文件解析
type Config struct {
	Port string `yaml:"Port"`
	Cos  `yaml:"Cos"`
	Oss  `yaml:"Oss"`
	Ups  `yaml:"Ups"`
}

type Cos struct {
	SecretID   string `yaml:"SecretID"`   //API密钥ID
	SecretKey  string `yaml:"SecretKey"`  //API密钥私钥
	Bucket     string `yaml:"Bucket"`     //存储桶名称 规则 test-1234567889
	Region     string `yaml:"Region"`     //存储桶所属地域 规则 ap-nanjing
	Domain     string `yaml:"Domain"`     //自定义域名
	APIAddress string `yaml:"APIAddress"` //API地址(访问域名) 在存储桶列表->配置管理->基础配置中可见 规则 https://<bucket>.cos.<region>.myqcloud.com
}

type Oss struct {
	Ak       string `yaml:"Ak"`       //AccessKey ID
	Sk       string `yaml:"Sk"`       //Access Key Secret
	Bucket   string `yaml:"Bucket"`   //Bucket
	Endpoint string `yaml:"Endpoint"` //外网访问地域节点(非Bucket域名)
	Domain   string `yaml:"Domain"`   //自定义域名(Bucket域名或自定义)
}

type Ups struct {
	Bucket   string `yaml:"Bucket"`   //服务名称
	Operator string `yaml:"Operator"` //授权的操作员名称
	Password string `yaml:"Password"` //授权的操作员密码
	Domain   string `yaml:"Domain"`   //加速域名
}

// ListObject 对象列表
type ListObject struct {
	Filename   string      `json:"filename"`
	Prefix     string      `json:"prefix"`
	IsDir      bool        `json:"is_dir"`
	Size       interface{} `json:"size"`
	CreateTime interface{} `json:"create_time"`
}

var (
	CosClient *cos.Client
	OssClient *oss.Bucket
)

func init() {
	//初始化配置
	GetConfig()
}

func UpsHandler(r *http.Request) (response []byte) {
	var up = upyun.NewUpYun(&upyun.UpYunConfig{
		Bucket:   config.Ups.Bucket,
		Operator: config.Ups.Operator,
		Password: config.Ups.Password,
	})
	//执行何种操作
	var operate = r.URL.Query().Get("operate")
	if operate == "list" {
		var path = r.URL.Query().Get("path")
		// path 为空 默认根目录
		if path == "" {
			path = "/"
		}
		objsChan := make(chan *upyun.FileInfo, 10)
		go func() {
			up.List(&upyun.GetObjectsConfig{
				Path:        path,
				ObjectsChan: objsChan,
			})
		}()
		var list []*upyun.FileInfo
		for obj := range objsChan {
			list = append(list, obj)
		}
		//返回信息
		response, _ = json.Marshal(&List{
			Code:    200,
			Message: config.Cos.Domain,
			Data:    list,
			Count:   len(list),
		})
	} else if operate == "delete" {
		//需要删除的文件绝对路径
		var path = r.URL.Query().Get("path")
		//执行删除
		if err := up.Delete(&upyun.DeleteObjectConfig{
			Path:  path,
			Async: false,
		}); err != nil {
			//删除失败
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorDelete:" + err.Error(),
			})
			return
		}
		response, _ = json.Marshal(&Response{
			Code:    200,
			Message: "ok",
		})
	} else if operate == "upload" {
		var _, header, err = r.FormFile("file")
		var path string
		r.ParseMultipartForm(32 << 20)
		if r.MultipartForm != nil {
			values := r.MultipartForm.Value["path"]
			if len(values) > 0 {
				path = values[0]
			}
		}
		if err != nil {
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorUpload:" + err.Error(),
			})
			return
		}
		dst := header.Filename
		source, _ := header.Open()
		if err := up.Put(&upyun.PutObjectConfig{
			Path:   path + dst,
			Reader: source,
		}); err != nil {
			//上传失败
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorUpload:" + err.Error(),
			})
			return
		}
		response, _ = json.Marshal(&Response{
			Code:    200,
			Message: "ok",
			Data:    config.Cos.Domain + path + dst,
		})
	} else if operate == "mkdir" {
		var dir = r.URL.Query().Get("dir")
		if err := up.Mkdir(dir); err != nil {
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorMkdir:" + err.Error(),
			})
			return
		}
		response, _ = json.Marshal(&Response{
			Code:    200,
			Message: "ok",
		})
	} else if operate == "domain" {
		response, _ = json.Marshal(&Response{
			Code:    200,
			Message: config.Cos.Domain,
		})
	}
	return
}

func InitCosClient() {
	config.Cos.APIAddress = fmt.Sprintf("https://%s.cos.%s.myqcloud.com", config.Cos.Bucket, config.Cos.Region)
	u, _ := url.Parse(config.APIAddress)
	b := &cos.BaseURL{BucketURL: u}
	CosClient = cos.NewClient(b, &http.Client{
		//设置超时时间
		Timeout: 100 * time.Second,
		Transport: &cos.AuthorizationTransport{
			//如实填写账号和密钥，也可以设置为环境变量
			SecretID:  config.Cos.SecretID,
			SecretKey: config.Cos.SecretKey,
		},
	})
}

func CosHandler(r *http.Request) (response []byte) {
	InitCosClient()
	var operate = r.URL.Query().Get("operate")
	if operate == "list" {
		// 列举当前目录下的所有文件
		var result []ListObject //结果集
		//设置筛选器
		var prefix = r.URL.Query().Get("prefix")
		opt := &cos.BucketGetOptions{
			Prefix:    prefix,
			Delimiter: "/",
			Marker:    prefix,
		}
		//结果入 result
		v, _, err := CosClient.Bucket.Get(context.Background(), opt)
		if err != nil {
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorListObject:" + err.Error(),
			})
			return
		}
		for _, dirname := range v.CommonPrefixes {
			result = append(result, ListObject{
				Filename:   strings.Replace(dirname, prefix, "", 1),
				CreateTime: "",
				IsDir:      true,
				Prefix:     prefix,
			})
		}
		for _, obj := range v.Contents {
			result = append(result, ListObject{
				Filename:   strings.Replace(obj.Key, prefix, "", 1),
				CreateTime: obj.LastModified,
				IsDir:      false,
				Prefix:     prefix,
				Size:       obj.Size,
			})
		}

		var domain string
		if config.Cos.Domain == "" {
			domain = config.APIAddress + "/"
		} else {
			domain = config.Cos.Domain
		}
		response, _ = json.Marshal(&List{
			Code:    200,
			Message: domain,
			Data:    result,
			Count:   len(result),
		})
	} else if operate == "delete" {
		//需要删除的文件绝对路径
		var path = r.URL.Query().Get("path")
		_, err := CosClient.Object.Delete(context.Background(), path)
		if err != nil {
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorObjectDelete:" + err.Error(),
			})
			return
		}
		response, _ = json.Marshal(&Response{
			Code:    200,
			Message: "ok",
		})
	} else if operate == "upload" {
		var _, header, err = r.FormFile("file")
		var prefix string
		_ = r.ParseMultipartForm(32 << 20)
		if r.MultipartForm != nil {
			values := r.MultipartForm.Value["prefix"]
			if len(values) > 0 {
				prefix = values[0]
			}
		}
		if err != nil {
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorUpload:" + err.Error(),
			})
			return
		}
		dst := header.Filename
		source, _ := header.Open()
		_, err = CosClient.Object.Put(context.Background(), prefix+dst, source, nil)
		if err != nil {
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorObjectUpload:" + err.Error(),
			})
			return
		}
		var domain string
		if config.Cos.Domain == "" {
			domain = config.APIAddress + "/"
		} else {
			domain = config.Cos.Domain
		}
		response, _ = json.Marshal(&Response{
			Code:    200,
			Message: "ok",
			Data:    domain + prefix + dst,
		})
	} else if operate == "domain" {
		var domain string
		if config.Cos.Domain == "" {
			domain = config.APIAddress + "/"
		} else {
			domain = config.Cos.Domain
		}
		response, _ = json.Marshal(&Response{
			Code:    200,
			Message: domain,
		})
	} else if operate == "mkdir" {
		var prefix = r.URL.Query().Get("prefix")
		var dirname = r.URL.Query().Get("dirname")
		_, err := CosClient.Object.Put(context.Background(), prefix+dirname, nil, nil)
		if err != nil {
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorMkdir:" + err.Error(),
			})
			return
		}
		response, _ = json.Marshal(&Response{
			Code:    200,
			Message: "ok",
		})
	}
	return
}

// Init 初始化操作
func InitOssClient() *Response {
	client, err := oss.New(config.Oss.Endpoint, config.Oss.Ak, config.Oss.Sk)
	if err != nil {
		return &Response{
			Code:    500,
			Message: "ErrorInitClient:" + err.Error(),
		}
	}
	// 获取存储空间。
	OssClient, err = client.Bucket(config.Oss.Bucket)
	if err != nil {
		return &Response{
			Code:    500,
			Message: "ErrorInitBucket:" + err.Error(),
		}
	}
	return nil
}
func OssHandler(r *http.Request) (response []byte) {
	if err := InitOssClient(); err != nil {
		response, _ = json.Marshal(err)
		return
	}
	var operate = r.URL.Query().Get("operate")
	if operate == "list" {
		// 列举当前目录下的所有文件
		var result []ListObject //结果集
		//设置筛选器
		var path = r.URL.Query().Get("prefix")
		maker := oss.Marker(path)
		prefix := oss.Prefix(path)
		//结果入 result
		for {
			lsRes, err := OssClient.ListObjects(maker, prefix, oss.Delimiter("/"))
			if err != nil {
				response, _ = json.Marshal(&Response{
					Code:    500,
					Message: "ErrorListObject:" + err.Error(),
				})
				return
			}
			for _, dirname := range lsRes.CommonPrefixes {
				result = append(result, ListObject{
					Filename:   strings.Replace(dirname, path, "", 1),
					CreateTime: time.Time{},
					IsDir:      true,
					Prefix:     path,
				})
			}
			for _, obj := range lsRes.Objects {
				result = append(result, ListObject{
					Filename:   strings.Replace(obj.Key, path, "", 1),
					CreateTime: obj.LastModified,
					IsDir:      false,
					Prefix:     path,
					Size:       obj.Size,
				})
			}
			prefix = oss.Prefix(lsRes.Prefix)
			maker = oss.Marker(lsRes.NextMarker)
			if !lsRes.IsTruncated {
				break
			}
		}
		response, _ = json.Marshal(&List{
			Code:    200,
			Message: config.Oss.Domain,
			Data:    result,
			Count:   len(result),
		})
	} else if operate == "delete" {
		//需要删除的文件绝对路径
		var path = r.URL.Query().Get("path")
		err := OssClient.DeleteObject(path)
		if err != nil {
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorObjectDelete:" + err.Error(),
			})
			return
		}
		response, _ = json.Marshal(&Response{
			Code:    200,
			Message: "ok",
		})
	} else if operate == "upload" {
		var _, header, err = r.FormFile("file")
		var prefix string
		_ = r.ParseMultipartForm(32 << 20)
		if r.MultipartForm != nil {
			values := r.MultipartForm.Value["prefix"]
			if len(values) > 0 {
				prefix = values[0]
			}
		}
		if err != nil {
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorUpload:" + err.Error(),
			})
			return
		}
		dst := header.Filename
		source, _ := header.Open()
		err = OssClient.PutObject(prefix+dst, source)
		if err != nil {
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorObjectUpload:" + err.Error(),
			})
			return
		}
		response, _ = json.Marshal(&Response{
			Code:    200,
			Message: "ok",
			Data:    config.Oss.Domain + prefix + dst,
		})
	} else if operate == "domain" {
		response, _ = json.Marshal(&Response{
			Code:    200,
			Message: config.Oss.Domain,
		})
	} else if operate == "mkdir" {
		var prefix = r.URL.Query().Get("prefix")
		var dirname = r.URL.Query().Get("dirname")
		err := OssClient.PutObject(prefix+dirname, nil)
		if err != nil {
			response, _ = json.Marshal(&Response{
				Code:    500,
				Message: "ErrorMkdir:" + err.Error(),
			})
			return
		}
		response, _ = json.Marshal(&Response{
			Code:    200,
			Message: "ok",
		})
	}
	return
}
