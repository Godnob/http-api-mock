package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/Godnob/http-api-mock/console"
	"github.com/Godnob/http-api-mock/definition"
	"github.com/Godnob/http-api-mock/logging"
	"github.com/Godnob/http-api-mock/match"
	"github.com/Godnob/http-api-mock/notify"
	"github.com/Godnob/http-api-mock/persist"
	"github.com/Godnob/http-api-mock/route"
	"github.com/Godnob/http-api-mock/server"
	"github.com/Godnob/http-api-mock/translate"
	"github.com/Godnob/http-api-mock/utils"
	"github.com/Godnob/http-api-mock/vars"
	"github.com/Godnob/http-api-mock/vars/fakedata"
)

//ErrNotFoundDefaultPath if we can't resolve the current path
var ErrNotFoundDefaultPath = errors.New("We can't determinate the current path")

//ErrNotFoundAnyMock when we don't found any valid mock definition to load
var ErrNotFoundAnyMock = errors.New("No valid mock definition found")

func banner() {
	fmt.Println("HTTP API Mock v 1.0.0")
	fmt.Println("")

	fmt.Print(
		`		.---. .---.
               :     : o   :    me want request!
           _..-:   o :     :-.._    /
       .-''  '  ` + "`" + `---' ` + "`" + `---' "   ` + "`" + `` + "`" + `-.
     .'   "   '  "  .    "  . '  "  ` + "`" + `.
    :   '.---.,,.,...,.,.,.,..---.  ' ;
    ` + "`" + `. " ` + "`" + `.                     .' " .'
     ` + "`" + `.  '` + "`" + `.                   .' ' .'
      ` + "`" + `.    ` + "`" + `-._           _.-' "  .'  .----.
        ` + "`" + `. "    '"--...--"'  . ' .'  .'  o   ` + "`" + `.
        .'` + "`" + `-._'    " .     " _.-'` + "`" + `. :       o  :
      .'      ` + "`" + `` + "`" + `` + "`" + `--.....--'''    ' ` + "`" + `:_ o       :
    .'    "     '         "     "   ; ` + "`" + `.;";";";'
   ;         '       "       '     . ; .' ; ; ;
  ;     '         '       '   "    .'      .-'
  '  "     "   '      "           "    _.-'
 `)
	fmt.Println("")
}

// Get preferred outbound ip of this machine
func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	logging.Println("Getting external IP")
	localAddr := conn.LocalAddr().String()
	idx := strings.LastIndex(localAddr, ":")

	return localAddr[0:idx]
}

func getRouter(mocks []definition.Mock, dUpdates chan []definition.Mock) *route.RequestRouter {
	logging.Printf("Loding router with %d definitions\n", len(mocks))
	router := route.NewRouter(mocks, match.MockMatch{}, dUpdates)
	router.MockChangeWatch()
	return router
}

func loadVarsProcessorEngines(persistPath string) *persist.PersistEngineBag {
	var persister persist.EntityPersister
	if strings.Index(persistPath, "mongodb://") < 0 {
		persister = persist.NewFilePersister(persistPath)
	} else {
		persister = persist.NewMongoPersister(persistPath)
	}

	persistBag := persist.GetNewPersistEngineBag(persister)
	return persistBag
}

func getVarsProcessor(persistEngineBag *persist.PersistEngineBag) vars.VarsProcessor {

	return vars.VarsProcessor{FillerFactory: vars.MockFillerFactory{}, FakeAdapter: fakedata.FakeAdapter{}, PersistEngines: persistEngineBag}
}

func startServer(ip string, port int, done chan bool, router route.Router, mLog chan definition.Match, varsProcessor vars.VarsProcessor, logs chan string) {
	dispatcher := server.Dispatcher{IP: ip,
		Port:          port,
		Router:        router,
		Translator:    translate.HTTPTranslator{},
		VarsProcessor: varsProcessor,
		Mlog:          mLog,
		Notifier:      notify.NewMockNotifier(),
		Logs:          logs,
	}
	dispatcher.Start()
	done <- true
}
func startConsole(ip string, port int, done chan bool, mLog chan definition.Match, logs chan string) {
	dispatcher := console.Dispatcher{IP: ip, Port: port, Mlog: mLog, Logs: logs}
	dispatcher.Start()
	done <- true
}

func getMocks(path string, updateCh chan []definition.Mock) []definition.Mock {
	logging.Printf("Reading Mock definition from: %s\n", path)

	definitionReader := definition.NewFileDefinition(path, updateCh)

	definitionReader.AddConfigReader(definition.JSONReader{})
	definitionReader.AddConfigReader(definition.YAMLReader{})

	mocks := definitionReader.ReadMocksDefinition()
	if len(mocks) == 0 {
		logging.Fatalln(ErrNotFoundAnyMock.Error())
	}
	definitionReader.WatchDir()
	return mocks
}

func main() {
	banner()
	outIP := getOutboundIP()
	path, err := filepath.Abs("./config")
	if err != nil {
		panic(ErrNotFoundDefaultPath)
	}

	persistPath, _ := filepath.Abs("./data")
	//persistPath := "mongodb://localhost/http-api-mock"

	sIP := flag.String("server-ip", outIP, "Mock server IP")
	sPort := flag.Int("server-port", 8083, "Mock Server Port")
	cIP := flag.String("console-ip", outIP, "Console Server IP")
	cPort := flag.Int("console-port", 8082, "Console server Port")
	console := flag.Bool("console", true, "Console enabled  (true/false)")
	cPath := flag.String("config-path", path, "Mocks definition folder")
	cPersistPath := flag.String("config-persist-path", persistPath, "Path to the folder where requests can be persisted or connection string to mongo database starting with mongodb:// and having database at the end /DatabaseName")

	flag.Parse()
	path, _ = filepath.Abs(*cPath)

	if strings.Index(*cPersistPath, "mongodb://") < 0 {
		*cPersistPath, _ = filepath.Abs(*cPersistPath)
	}

	//chanels
	mLog := make(chan definition.Match)
	logs := make(chan string)
	dUpdates := make(chan []definition.Mock)
	done := make(chan bool)

	mocks := getMocks(path, dUpdates)
	router := getRouter(mocks, dUpdates)

	persistEngineBag := loadVarsProcessorEngines(*cPersistPath)
	varsProcessor := getVarsProcessor(persistEngineBag)

	go startServer(*sIP, *sPort, done, router, mLog, varsProcessor, logs)

	utils.SetServerAddress(fmt.Sprintf("http://%s:%d", *sIP, *sPort))

	logging.Printf("HTTP Server running at %s\n", utils.GetServerAddress())

	if *console {
		go startConsole(*cIP, *cPort, done, mLog, logs)
		logging.Printf("Console running at http://%s:%d\n", *cIP, *cPort)

		logging.SetLogger(logging.ChannelLogger{ChannelLog: logs})
	}

	<-done

}
