package mobile

import "encoding/json"

type envelope struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data"`
	Error *apiError   `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type request struct {
	Op            string `json:"op"`
	WorkspacePath string `json:"workspacePath"`
	IndexPath     string `json:"indexPath"`
	View          string `json:"view"`
	Context       string `json:"context"`
}

func decodeRequest(req string) (request, error) {
	var r request
	err := json.Unmarshal([]byte(req), &r)
	return r, err
}

func encodeOK(data interface{}) string {
	if data == nil {
		data = map[string]interface{}{}
	}
	b, err := json.Marshal(envelope{OK: true, Data: data, Error: nil})
	if err != nil {
		return encodeFail("internal", "failed to encode response")
	}
	return string(b)
}

func encodeFail(code, message string) string {
	b, err := json.Marshal(envelope{
		OK:   false,
		Data: nil,
		Error: &apiError{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		return `{"ok":false,"data":null,"error":{"code":"internal","message":"failed to encode response"}}`
	}
	return string(b)
}
