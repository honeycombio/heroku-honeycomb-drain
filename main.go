package main

import (
	"log"
	"net/http"
	"os"
	"fmt"
	"bytes"
	"time"
	"bufio"
	"strings"
	"encoding/json"
	"github.com/go-logfmt/logfmt"
	"github.com/bmizerany/lpx"
	"github.com/honeycombio/libhoney-go"
)

type LogDrain struct {
	DebugLogs bool
	AllowedApps map[string]string
	AppFormats [][2]string
}

type ToEvent func(message []byte, event *libhoney.Event) bool

var formatterMap = map[string]ToEvent{
	"logfmt": LogFmtToEvent,
	"json": JsonToEvent,
	"raw": RawToEvent,
	"ignore": IgnoreToEvent,
}
// heroku/router:logfmt
// heroku/*:logfmt
// foo/web:json

func JsonToEvent(message []byte, event *libhoney.Event) bool {
	var j map[string]interface{}
	err := json.Unmarshal(message, &j)
	if err != nil {
		event.AddField("json_err", err)
		event.AddField("raw_message", string(message))
		return true
	} else {		
		for k, v := range j {
			event.AddField(k, v)
		}
	}
	
	return true
}

func LogFmtToEvent(message []byte, event *libhoney.Event) bool {
	d := logfmt.NewDecoder(bytes.NewBuffer(message)) 
	for d.ScanRecord() {
		for d.ScanKeyval() {
			event.AddField(string(d.Key()), string(d.Value())) 
		}
		
		if d.Err() != nil {
			event.AddField("logfmt_err", d.Err()) 
			event.AddField("raw_message", string(message))
		}
	}
	
	return true
}

func RawToEvent(message []byte, event *libhoney.Event) bool {
	event.AddField("message", string(message))
	return true
}

func IgnoreToEvent(message []byte, event *libhoney.Event) bool {
	return false
}

func (ld *LogDrain) FormatterForHostApp(name string, proc_id string) ToEvent {
	variants := [2]string{
		fmt.Sprintf("%s/%s", name, proc_id), 
		fmt.Sprintf("%s/*", name),
	}
		
	for _, spec := range ld.AppFormats {
		for _, variant := range variants {
			if spec[0] == variant {
				return formatterMap[spec[1]]
			}
		}
	}
	
	return RawToEvent
}

func (ld *LogDrain) Handle(w http.ResponseWriter, req *http.Request) {
	username, password, ok := req.BasicAuth()
	if len(ld.AllowedApps) > 0 {
		expectedPassword := ld.AllowedApps[username]
		if ok && password != expectedPassword {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}
	
	
	builder := libhoney.NewBuilder() 

	lp := lpx.NewReader(bufio.NewReader(req.Body)) 
	for lp.Next() {
		event := builder.NewEvent() 
		
		name, proc_id := string(lp.Header().Name), string(lp.Header().Procid)
		
		event.AddField("host", string(lp.Header().Hostname)) 
		event.AddField("app", name) 
		event.AddField("proc_id", proc_id)
		event.AddField("msg_id", string(lp.Header().Msgid)) 
		
		formatter := ld.FormatterForHostApp(name, proc_id)
		
		emitEvent := formatter(lp.Bytes(), event)
			
		if !emitEvent {
			return
		}
		
		ts, err := time.Parse(time.RFC3339, string(lp.Header().Time))
		if err != nil {
			event.AddField("time_error", err) 
			ts = time.Now() 
		}
		
		event.Timestamp = ts 

		if(!ld.DebugLogs) {
			event.Send()
		} else {
			eventJson, err := json.Marshal(event) 
			if err != nil {
				log.Println(err)
				continue
			}
			log.Println(string(eventJson)) 
		}
	}
	w.WriteHeader(http.StatusOK) 
}

func makeAppFormats(appFormats string) [][2]string {
	var appFormatsArray [][2]string
	for _, spec := range strings.Split(appFormats, ",") {
		if len(spec) == 0 {
			log.Fatal("No APP_FORMATS specified.")
		}
		
		splitSpec := strings.Split(spec, ":")
		
		if len(splitSpec) < 2 {
			log.Fatalf("spec '%s' missing an app or a format", spec)
		}
		name, format := splitSpec[0], splitSpec[1]
				
		appFormatsArray = append(appFormatsArray, [2]string{name, format})
	}
	
	return appFormatsArray
}

func makeAllowedApps(allowedApps string) map[string]string {
	allowedAppsMap := make(map[string]string)
	
	for _, creds := range strings.Split(allowedApps, ",") {
		if creds == "" {
			continue
		}
		splitCreds := strings.Split(creds, ":")
		if len(splitCreds) != 2 {
			log.Fatalf("creds should be in the form of 'user:pass', got %s", creds)
		}
		allowedAppsMap[splitCreds[0]] = splitCreds[1]
	}
	
	return allowedAppsMap
}

func main() {
	allowedApps := os.Getenv("ALLOWED_APPS")
	port := os.Getenv("PORT")
	dataSet := os.Getenv("HONEYCOMB_DATASET")
	writeKey := os.Getenv("HONEYCOMB_WRITE_KEY") 
	appFormats := os.Getenv("APP_FORMATS")

	if dataSet == "" {
		dataSet = "heroku-logdrain"
	}
	
	libhoney.Init(libhoney.Config{
  	WriteKey: writeKey,
  	Dataset: dataSet,
	}) 
	defer libhoney.Close() 

	if port == "" {
		log.Fatal("$PORT must be set")
	}
	
	if writeKey != "" && allowedApps == "" {
		log.Fatal("$ALLOWED_APPS must be set if $HONEYCOMB_WRITE_KEY is set.")
	}
	
	ld := &LogDrain{
		writeKey == "",
		makeAllowedApps(allowedApps),
		makeAppFormats(appFormats),
	}
	
	http.HandleFunc("/", ld.Handle) 
	log.Fatal(http.ListenAndServe(":" + port, nil)) 
}
