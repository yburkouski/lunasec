package tokenizer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/refinery-labs/loq/model"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/refinery-labs/loq/constants"
	"github.com/refinery-labs/loq/model/event"
	"github.com/refinery-labs/loq/service"
	"github.com/urfave/cli"
	"go.uber.org/zap"
)

type CliOptions struct {
	URL                string
	Secret             string
	Token              string
	Plaintext          string
	Input              string
	Output             string
	Metadata           string
	CustomerPrivateKey string
}

func cliOptionsStruct(c *cli.Context) CliOptions {
	return CliOptions{
		URL:                c.GlobalString("url"),
		CustomerPrivateKey: c.GlobalString("customer-private-key"),
		Secret:             c.String("secret"),
		Token:              c.String("token"),
		Plaintext:          c.String("plaintext"),
		Input:              c.String("input"),
		Output:             c.String("output"),
		Metadata:           c.String("metadata"),
	}
}

func newJwtSigner(customerPrivateKey string) service.JwtSigner {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	decodedPrivateKey, err := ioutil.ReadFile(customerPrivateKey)
	if err != nil {
		panic(err)
	}
	jwtSigner, err := service.NewJwtSignerFromPrivateKey(logger, decodedPrivateKey)
	if err != nil {
		panic(err)
	}
	return jwtSigner
}

func newAuthJwt(sessionID string, customerPrivateKey string) string {
	jwtSigner := newJwtSigner(customerPrivateKey)
	claims := model.SessionJwtClaims{
		SessionID: sessionID,
	}
	token, err := jwtSigner.CreateWithSessionClaims(claims)
	if err != nil {
		panic(err)
	}
	return token
}

func tokenizerRequest(sessionID string, url, customerPrivateKey string, input interface{}) (data []byte, err error) {
	reqBody, err := json.Marshal(input)
	if err != nil {
		log.Println(err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Println(err)
		return
	}

	auth := newAuthJwt(sessionID, customerPrivateKey)

	req.Header.Add(constants.JwtAuthHeader, auth)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func s3Upload(url string, headers map[string]string, body []byte) (data []byte, err error) {
	return s3Request(http.MethodPut, url, headers, bytes.NewBuffer(body))
}

func s3Download(url string, headers map[string]string) (data []byte, err error) {
	return s3Request(http.MethodGet, url, headers, bytes.NewBuffer([]byte{}))
}

func s3Request(method, url string, headers map[string]string, body *bytes.Buffer) (data []byte, err error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Println(err)
		return
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func newSessionID() string {
	sessionID := uuid.New()
	return sessionID.String()
}

func setGrantForToken(cliOptions CliOptions, sessionID string, tokenID string) (err error) {
	input := event.GrantSetRequest{
		TokenID: tokenID,
		GrantType: constants.ReadToken,
	}
	tokenizeURL := fmt.Sprintf("%s/grant/set", cliOptions.URL)
	_, err = tokenizerRequest(sessionID, tokenizeURL, cliOptions.CustomerPrivateKey, input)
	if err != nil {
		log.Println(err)
		return
	}
	return
}

func CreateJwtAuthCommand(c *cli.Context) (err error) {
	cliOptions := cliOptionsStruct(c)

	jwtAuth := newAuthJwt(newSessionID(), cliOptions.CustomerPrivateKey)
	log.Printf("jwt: %s", jwtAuth)
	return
}

func TokenizeCommand(c *cli.Context) (err error) {
	var (
		content []byte
		resp    struct {
			Success bool
			Data    event.TokenizerSetResponse
		}
	)
	cliOptions := cliOptionsStruct(c)

	if cliOptions.Plaintext != "" {
		content = []byte(cliOptions.Plaintext)
	} else if cliOptions.Input != "" {
		content, err = ioutil.ReadFile(cliOptions.Input)
		if err != nil {
			return
		}
	} else {
		err = errors.New("neither option '--plaintext' or '--input' was provided")
		return
	}

	sessionID := newSessionID()

	input := event.TokenizerSetRequest{}
	tokenizeURL := fmt.Sprintf("%s/tokenize", cliOptions.URL)
	data, err := tokenizerRequest(sessionID, tokenizeURL, cliOptions.CustomerPrivateKey, input)
	if err != nil {
		log.Println(err)
		return
	}

	err = json.Unmarshal(data, &resp)
	if err != nil {
		log.Println(err)
		return
	}

	if !resp.Success {
		err = errors.New("server was unable to tokenize token")
		return
	}

	fmt.Printf("%v", resp.Data)

	s3Resp, err := s3Upload(resp.Data.UploadURL, resp.Data.Headers, content)
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf("s3 response: %s", string(s3Resp))
	log.Printf("token: %s", resp.Data.TokenID)
	return
}

func DetokenizeCommand(c *cli.Context) (err error) {
	var (
		resp struct {
			Success bool
			Data    event.TokenizerGetResponse
		}
	)
	cliOptions := cliOptionsStruct(c)

	input := event.TokenizerGetRequest{}
	tokenID := cliOptions.Token

	input.TokenID = tokenID

	sessionID := newSessionID()

	err = setGrantForToken(cliOptions, sessionID, tokenID)
	if err != nil {
		log.Println(err)
		return
	}

	detokenizeURL := fmt.Sprintf("%s/detokenize", cliOptions.URL)
	data, err := tokenizerRequest(sessionID, detokenizeURL, cliOptions.CustomerPrivateKey, input)
	if err != nil {
		log.Println(err)
		return
	}

	err = json.Unmarshal(data, &resp)
	if err != nil {
		log.Println(err)
		return
	}

	if !resp.Success {
		err = errors.New("server was unable to detokenize token")
		return
	}

	s3Resp, err := s3Download(resp.Data.DownloadURL, resp.Data.Headers)
	if err != nil {
		log.Println(err)
		return
	}
	if cliOptions.Output != "" {
		err = ioutil.WriteFile(cliOptions.Output, s3Resp, 0755)
		if err != nil {
			return
		}
	} else {
		log.Printf("s3 response: %s", s3Resp)
	}
	return
}

func SetMetadataCommand(c *cli.Context) (err error) {
	var (
		resp struct {
			Success bool
			Data    event.MetadataSetResponse
		}

		metadata map[string]interface{}
	)
	cliOptions := cliOptionsStruct(c)

	err = json.Unmarshal([]byte(cliOptions.Metadata), &metadata)
	if err != nil {
		log.Println(err)
		return err
	}

	tokenID := cliOptions.Token

	sessionID := newSessionID()

	err = setGrantForToken(cliOptions, sessionID, tokenID)
	if err != nil {
		log.Println(err)
		return
	}

	input := event.MetadataSetRequest{
		TokenID:  tokenID,
		Metadata: metadata,
	}
	metadataSetURL := fmt.Sprintf("%s/metadata/set", cliOptions.URL)
	data, err := tokenizerRequest(sessionID, metadataSetURL, cliOptions.CustomerPrivateKey, input)
	if err != nil {
		log.Println(err)
		return
	}
	err = json.Unmarshal(data, &resp)
	if err != nil {
		log.Println(err)
		return
	}
	if !resp.Success {
		err = errors.New("server was unable to set metadata for token")
		return
	}
	log.Printf("setting metdata for %s was successful", cliOptions.Token)
	return
}

func GetMetadataCommand(c *cli.Context) (err error) {
	var (
		resp struct {
			Success bool
			Data    event.MetadataGetResponse
		}
	)
	cliOptions := cliOptionsStruct(c)

	tokenID := cliOptions.Token

	sessionID := newSessionID()

	err = setGrantForToken(cliOptions, sessionID, tokenID)
	if err != nil {
		log.Println(err)
		return
	}

	input := event.MetadataGetRequest{
		TokenID: tokenID,
	}
	metadataGetURL := fmt.Sprintf("%s/metadata/get", cliOptions.URL)
	data, err := tokenizerRequest(sessionID, metadataGetURL, cliOptions.CustomerPrivateKey, input)
	if err != nil {
		log.Println(err)
		return
	}
	err = json.Unmarshal(data, &resp)
	if err != nil {
		log.Println(err)
		return
	}
	if !resp.Success {
		err = errors.New("server was unable to get metadata for token")
		return
	}
	log.Printf("metdata for %s: %v", cliOptions.Token, resp.Data.Metadata)
	return
}