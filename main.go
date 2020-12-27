package main

import (
	"crypto"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cavaliercoder/grab"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

var client = &http.Client{}

const BaseAPIURL = "https://api.modpacks.ch/"
const BaseModpackURL = BaseAPIURL + "public/modpack/"
const SearchURL = BaseModpackURL + "search/5?term="
var (
	inProgress = 0
	succeeded = 0
	failed = 0
	parsedArgs = make(map[string]interface{})
)

/*func initWindows() {
	mode := uint32(0)
	if err := windows.GetConsoleMode(windows.Stdout, &mode); err != nil {
		return
	}

	mode = mode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	_ = windows.SetConsoleMode(windows.Stdout, mode)
}*/

func main() {
	//initWindows() // todo: more work on making it look nice and work properly, and avoid doing it unless windows 10
	filename := filepath.Base(os.Args[0])
	filename = "serverinstall_61_191"
	var (
		auto = false
		tempPath = ""
		startScript = false
		help = false
	)
	flag.BoolVar(&auto, "auto", auto, "Ask no questions, use defaults.")
	flag.StringVar(&tempPath, "path", tempPath, "Directory to install in. Default: current `directory`")
	flag.BoolVar(&startScript, "startScript", startScript, "Generate a start script using specifications. Default: false")
	flag.BoolVar(&help,"help", help, "This help screen")
	flag.Parse()
	parsedArgs["auto"] = auto
	parsedArgs["path"] = tempPath
	parsedArgs["startScript"] = startScript
	if help {
		flag.Usage()
		os.Exit(0)
	}
	HandleLaunch(filename)
}

/*func Search(term string) []Modpack {
	termSafe := url.QueryEscape(term)
	result := SearchResult{}
	APICall(SearchURL + termSafe, *result)
	return []Modpack{}
}*/

func HandleLaunch(file string) {
	var installPath = parsedArgs["path"].(string)
	if len(installPath) == 0 {
		response := QuestionFree("current directory", "Where would you like to install the server?")
		if response != "current directory" {
			installPath = response
		}
	}
	if len(installPath) == 0 || installPath[0] != "/"[0] {
		installPath = path.Join(".", installPath)
	}
	if _, err := os.Stat(installPath); os.IsNotExist(err) {
		if QuestionYN(true,"Path %s does not exist - want to create it?", installPath) {
			if err := os.MkdirAll(installPath, os.FileMode(755)); err != nil {
				log.Fatalf("An error occured whilst creating the folder %s: %v", installPath, err)
			}
		}
	}
	upgrade := false
	if _, err := os.Stat(path.Join(installPath, "Version.json")); !os.IsNotExist(err) {
		upgrade = true
	}
	_ = upgrade // temp
	re := regexp.MustCompile("^serverinstall_(\\d+)_(\\d+)")
	matched := re.FindStringSubmatch(file)
	modpackid, err := strconv.Atoi(matched[1])
	if err != nil {
		log.Fatalf("Cannot locate modpack via filename: %s", file)
	}
	versionid, err := strconv.Atoi(matched[2])
	if err != nil {
		log.Fatalf("Cannot locate modpack via filename: %s", file)
	}
	modpack := GetModpack(modpackid)
	if modpack.Status == "error" {
		log.Fatalf("Error fetching modpack: %s", modpack.Message)
	}
	var version *Version
	Free:
	for _, v := range modpack.Versions {
		if v.ID == versionid {
			version = &v
			break Free
		}
	}
	if version == nil {
		log.Fatalf("Error fetching modpack: %s", "Version does not exist")
	}

	versionInfo := GetVersion(modpack, *version)
	if versionInfo.Status == "error" {
		log.Fatalf("Error fetching modpack: %s", modpack.Message)
	}

	var downloads []Download

	for _, v := range versionInfo.Files {
		if v.ClientOnly {
			continue
		}
		parse, err := url.Parse(v.URL)
		if err != nil {
			//shrug
			continue
		}
		downloads = append(downloads, Download{v.Path, *parse, v.Name, v.SHA1})
	}

	var modloader Target
	var minecraftTar Target

	for _, v := range versionInfo.Targets {
		if v.Type == "modloader" {
			modloader = v
		}
		if v.Type == "game" {
			minecraftTar = v
		}
	}

	minecraft := Minecraft{}
	minecraft.RawVersion = minecraftTar.Version
	if err := minecraft.Parse(); err != nil {
		log.Fatalf("Error parsing Version: %v", err)
	}

	err, ml := GetModloader(modloader, minecraft)

	if err != nil {
		log.Fatalf("Error getting Modloader: %v", err)
	}

	modloaderDls := ml.GetDownloads(installPath)

	downloads = append(downloads, modloaderDls...)

	grabs, err := GetBatch(64, installPath, downloads...)
	if err != nil {
		log.Fatal(err)
	}
	responses := make([]*grab.Response, 0, len(downloads))
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()

	Loop:
	for {
		select {
		case resp := <-grabs:
			if resp != nil {
				// a new response has been received and has started downloading
				responses = append(responses, resp)
			} else {
				// channel is closed - all downloads are complete
				updateUI(responses)
				break Loop
			}

		case <-t.C:
			// update UI every 200ms
			updateUI(responses)
		}
	}

	fmt.Printf(
		"Finished %d successful, %d failed, %d incomplete.\n",
		succeeded,
		failed,
		inProgress)

	ml.Install(installPath)

	// return the number of failed downloads as exit code
	os.Exit(failed)
}

func GetModpack(id int) Modpack {
	modpack := Modpack{}
	newurl := fmt.Sprintf(BaseModpackURL+"%d", id)
	err, newmodpack := APICall(newurl)
	if err != nil {
		modpack := Modpack{}
		modpack.Status = "error"
		modpack.Message = err.Error()
		return modpack
	}
	err = json.Unmarshal(newmodpack, &modpack)
	if err != nil {
		modpack := Modpack{}
		modpack.Status = "error"
		modpack.Message = err.Error()
		return modpack
	}
	return modpack
}

func GetVersion(modpack Modpack, version Version) VersionInfo {
	newUrl := fmt.Sprintf(BaseModpackURL+"%d/%d", modpack.ID, version.ID)
	err, resp := APICall(newUrl)
	newversion := VersionInfo{}
	if err != nil {
		newversion.Status = "error"
		newversion.Message = err.Error()
		return newversion
	}
	err = json.Unmarshal(resp, &newversion)
	if err != nil {
		newversion = VersionInfo{}
		newversion.Status = "error"
		newversion.Message = err.Error()
		return newversion
	}
	return newversion
}

func APICall(url string) (error, []byte) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err, nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return err, nil
	}

	stringRet, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err, nil
	}

	resp.Body.Close()

	return nil, stringRet
}

func GetBatch(workers int, dst string, urlStrs ...Download) (<-chan *grab.Response, error) {
	fi, err := os.Stat(dst)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("destination is not a directory")
	}

	reqs := make([]*grab.Request, len(urlStrs))
	for i := 0; i < len(urlStrs); i++ {
		req, err := grab.NewRequest(path.Join(dst, urlStrs[i].Path, urlStrs[i].Name), urlStrs[i].URL.String())
		if err != nil {
			return nil, err
		}
		if len(urlStrs[i].SHA1) > 0 {
			byteHex, _ := hex.DecodeString(urlStrs[i].SHA1)
			req.SetChecksum(crypto.SHA1.New(), byteHex, false)
		}

		reqs[i] = req
	}

	ch := grab.DefaultClient.DoBatch(workers, reqs...)
	return ch, nil
}


func updateUI(responses []*grab.Response) {
	// clear lines for incomplete downloads
	if inProgress > 0 {
		fmt.Printf("\033[%dA\033[K", inProgress)
	}

	// print newly completed downloads
	for i, resp := range responses {
		if resp != nil && resp.IsComplete() {
			if resp.Err() != nil {
				failed++
				fmt.Fprintf(os.Stderr, "Error downloading %s: %v\n",
					resp.Request.URL(),
					resp.Err())
			} else {
				succeeded++
				fmt.Printf("Finished %s %d / %d bytes (%d%%)\n",
					resp.Filename,
					resp.BytesComplete(),
					resp.Size,
					int(100*resp.Progress()))
			}
			responses[i] = nil
		}
	}

	// print progress for incomplete downloads
	inProgress = 0
	for _, resp := range responses {
		if resp != nil {
			fmt.Printf("Downloading %s %d / %d bytes (%d%%) - %.02fKBp/s ETA: %ds \033[K\n",
				resp.Filename,
				resp.BytesComplete(),
				resp.Size,
				int(100*resp.Progress()),
				resp.BytesPerSecond()/1024,
				int64(resp.ETA().Sub(time.Now()).Seconds()))
			inProgress++
		}
	}
}