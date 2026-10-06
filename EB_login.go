package main

import (
	"strings"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/dronm/elkonexport/logger"
)

const (
	respOK ResponseCode = 0
	
	responseModelID ModelID = "ModelServResponse"	
	
	quoteChar byte = 34
)
var (
	errModelNotFound = errors.New("модель с рзультатом не найдена")
)

const httpView = "ViewJSON" 

type Response struct {
	Models map[ModelID]json.RawMessage `json:"models"`
}

type ResponseCode int
func (v *ResponseCode) UnmarshalJSON(data []byte) error {
	
	if string(data) == "null" {
		return nil
	}
	
	var tmp int64
	var err error
	if data[0] == quoteChar && byte(data[len(data)-1]) ==  quoteChar {
		tmp, err = strconv.ParseInt(string(data[1:len(data)-1]), 10, 32)
	}else{
		tmp, err = strconv.ParseInt(string(data), 10, 32)
	}	
	if err!=nil {
		return err
	}				
	*v = ResponseCode(tmp)
	
	return nil	
}

type ResponseModelFields struct {
	Code ResponseCode `json:"result"`
	Descr string `json:"descr"`
	QueryID string `json:"query_id"`
	AppVersion string `json:"app_version"`
}

type ResponseModel struct {
	ID ModelID `json:"id"`
	Rows []ResponseModelFields
}
func (m *ResponseModel) GetID() ModelID {
	return responseModelID
}


type ModelID string

type Model interface {
	GetID() ModelID
}

type AuthModelFields struct {
	AccessToken string `json:"access_token"`
	TokenExpires string `json:"tokenExpires"`
}

type AuthModel struct {
	ID ModelID `json:"id"`
	Rows []*AuthModelFields
}
func (m *AuthModel) GetID() ModelID {
	return ModelID("AuthModel")
}

type UserInfModelFields struct {
	Name string `json:"name"`
	TelExt string `json:"tel_ext"`
}


type UserInfModel struct {
	ID ModelID `json:"id"`
	Rows []*UserInfModelFields
}
func (m *UserInfModel) GetID() ModelID {
	return ModelID("UserInfModel")
}

func parseResponse(body []byte, models []Model) (error, ResponseCode) {
	srvResp := Response{}
	err := json.Unmarshal([]byte(body), &srvResp)
	if err != nil {
		return err, -1
	}

	respMs,ok := srvResp.Models[responseModelID]
	if !ok {
		return errModelNotFound, -1
	}
	respM := ResponseModel{}
	err = json.Unmarshal(respMs, &respM)
	if err != nil {
		return err, -1
	}
	if !ok || len(respM.Rows)==0 {
		return errModelNotFound, -1
	}
	if respM.Rows[0].Code != respOK {
		return errors.New(respM.Rows[0].Descr), respM.Rows[0].Code
	}
	
	//all requested models
	for _,model := range models {
		respMs,ok = srvResp.Models[model.GetID()]
		if ok {		
			if err := json.Unmarshal(respMs, model); err != nil {
				logger.Logger.Error(err)
			}
		}
	}
	
	return nil,0
}

//Sends http query, parses result, fills supplied midels from query data
func sendQuery(endpointURL string, params *url.Values, models...Model) error {

	url := endpointURL+"?" + params.Encode()
	logger.Logger.Debugf("sending http query: %s", url)
	
	resp, err := http.Get(url)
	if err != nil {		
		return err
	}
	
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	
	err,_ = parseResponse(body, models)
	return err
}

// macAddr gets the MAC hardware
// address of the host machine
func macAddr() string {
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, i := range interfaces {
			addrStr := i.HardwareAddr.String()
			if i.Flags&net.FlagUp != 0 && addrStr != "" {
				// Don't use random as we have a real address
				//: => -
				addr := strings.ReplaceAll(addrStr, ":", "-")
				addr = strings.ToUpper(addr); //all capital
				return addr
			}
		}
	}

	return ""
}
//**********************************************************

// Login logins with http and returns token to be used with websocket.
func Login(endpointURL string) (string, error) {

	mac := macAddr()
	if mac == "" {
		return "", errors.New("unable to get MAC")
	}
	logger.Logger.Debugf("MAC address: %s", mac)

	//MAC hash for login!
	hash := md5.Sum([]byte(mac))
	hashStr := hex.EncodeToString(hash[:])

	v := url.Values{}
	v.Add("c", "User_Controller")
	v.Add("f", "login_ks")
	v.Add("v", httpView)
	v.Add("k", hashStr)
	
	auth := AuthModel{}
	// user := UserInf_Model{}

	err := sendQuery(endpointURL, &v, &auth)
	if err != nil {
		return "", fmt.Errorf("sendQuery(): %v", err)
	}

	if len(auth.Rows) == 0 {
		return "", errors.New("access token model not found")
	}

	return auth.Rows[0].AccessToken, nil
}
