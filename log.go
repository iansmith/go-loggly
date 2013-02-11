package loggly

import (
  "net/http"
	"bytes"
	"encoding/json"
	"log"
	"os"
	"log/syslog"
	"fmt"
)

type Logger interface {
	Debugf(spec string, other ...interface{})
	Infof(spec string, other ...interface{})
	Warnf(spec string, other ...interface{})
	Errorf(spec string, other ...interface{})
	Fatalf(spec string, other ...interface{})
	WaitToDie()
	
	Error(error)
	ErrorPanic(error)
	ErrorFatal(error)
}

type LogglyLogger struct {
	out chan *logglyBundle
	in chan int
	thresh syslog.Priority
	t *log.Logger
}

type logglyBundle struct {
	Prefix string `json: "prefix"`
	Level syslog.Priority `json: "level"`
	Type string `json: "type"`
	Message string `json: "message"`
}


func NewEndpoint(logglyEndpoint string, prefix string, thresh syslog.Priority) *LogglyLogger {
	result:=&LogglyLogger{}
	p:="["+prefix+"]"
	if logglyEndpoint!="" {
		result.out=make(chan *logglyBundle, 10)  //buffers up to 10 messages before blocking
		result.in=make(chan int)
		go logSender(result.out, result.in, logglyEndpoint, p)
	} 
	result.thresh=thresh
	result.t = log.New(os.Stderr, p, log.Lshortfile)
	return result
}

func logSender(in chan *logglyBundle, out chan int, endpoint string, prefix string) {
	buff := bytes.Buffer{}
	enc := json.NewEncoder(&buff)
	client := http.Client{}
	for {
		bundle, open:= <- in
		if !open {
			break
		}
		bundle.Prefix = prefix
		buff.Reset()
		err:=enc.Encode(bundle)
		if err!=nil {
			fmt.Fprintf(os.Stderr,"Unable to encode message for loggly! %s\n",err.Error())
			continue
		}		
		resp,err:=client.Post(endpoint, "application/json", &buff)
		bundle=nil
		if err!=nil {
			fmt.Fprintf(os.Stderr,"Unable to post log message to loggly! %s\n",err.Error())
			continue
		}
		if resp.StatusCode!=200 {
			fmt.Fprintf(os.Stderr,"Error posting log message to loggly! %s\n",resp.Status)
			continue
		}
	}
	close(out)
}
func (self *LogglyLogger) send(prio syslog.Priority, t string, msg string) {
	if self.out!=nil {
		bundle := logglyBundle {
			Level: prio,
			Type: t,
			Message: msg,
		}
		self.out <- &bundle
	}
}

func (self *LogglyLogger) outf(prio syslog.Priority, spec string, other ...interface{}) {
	if self.t!=nil {
		if prio<=self.thresh {
			self.t.Printf(spec,other...)
		}
	}
	if self.out!=nil {
		self.send(prio, "", fmt.Sprintf(spec,other...))
	}
}

func (self *LogglyLogger) errorLevelf(prio syslog.Priority, where string, err error) {
	if self.t!=nil {
		if prio<=self.thresh {
			self.t.Printf("%T:%s:%v",err,where,err.Error())
		}
	}
	if self.out!=nil {
		self.send(prio, fmt.Sprintf("%T",err), fmt.Sprintf("%s",err.Error()))
	}
}

func (self *LogglyLogger) Debugf(spec string, other ...interface{}) {
	self.outf(syslog.LOG_DEBUG, spec, other...)
}

func (self *LogglyLogger) Infof(spec string, other ...interface{}) {
	self.outf(syslog.LOG_INFO, spec, other...)
}

func (self *LogglyLogger) Warnf(spec string, other ...interface{}) {
	self.outf(syslog.LOG_WARNING, spec, other...)
}

func (self *LogglyLogger) Noticef(spec string, other ...interface{}) {
	self.outf(syslog.LOG_NOTICE, spec, other...)
}

func (self *LogglyLogger) Errf(spec string, other ...interface{}) {
	self.outf(syslog.LOG_ERR, spec, other...)
}

func (self *LogglyLogger) Critf(spec string, other ...interface{}) {
	self.outf(syslog.LOG_CRIT, spec, other...)
}

func (self *LogglyLogger) Alertf(spec string, other ...interface{}) {
	self.outf(syslog.LOG_ALERT, spec, other...)
}

func (self *LogglyLogger) Emergf(spec string, other ...interface{}) {
	self.outf(syslog.LOG_EMERG, spec, other...)
}

func (self *LogglyLogger) Error(where string, err error) {
	self.errorLevelf(syslog.LOG_ERR, where, err)
}

func (self *LogglyLogger) ErrorPanic(where string, err error) {
	self.errorLevelf(syslog.LOG_ALERT, where, err)
	self.WaitToDie()
	panic(err)
}

func (self *LogglyLogger) ErrorFatal(where string, err error) {
	self.errorLevelf(syslog.LOG_EMERG, where, err)
	self.WaitToDie()
	os.Exit(1)
}

func (self *LogglyLogger) WaitToDie() {
	close(self.out)
	<- self.in
}
