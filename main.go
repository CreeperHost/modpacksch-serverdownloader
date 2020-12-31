package main

import (
	"crypto"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cavaliercoder/grab"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

var client = &http.Client{}

const BaseAPIURL = "https://api.modpacks.ch/"
const BaseModpackURL = BaseAPIURL + "public/modpack/"
const SearchURL = BaseModpackURL + "search/5?term="
const BaseName = "serverinstall"
const verStr = "1.0"
var (
	inProgress = 0
	succeeded = 0
	failed = 0
)

var Options struct {
	Auto            bool    `help:"Ask no questions, use defaults."`
	Path            string  `help:"Directory to install in. Default: current directory"`
	Noscript        bool    `help:"Skip creating start script. Default: false"`
	Threads         int     `help:"Number of threads to use for downloading. Default: cpucores * 2"`
	Integrityupdate bool    `help:"Whether changed files should be overwritten with fresh copies when updating. Most useful when used with Auto. Default: false"`
	Integrity       bool    `help:"Do a full integrity check. integrityUpdate assumed. Default: false"`
	Verbose         bool    `help:"Be a bit noisier on actions taken. Default: false"`
	Latest          bool    `help:"Install latest, ignoring any version in the file name. Default: false"`
	Help            bool    `help:"This help"`
}

/*flag.Bool(&Options.Auto, "-Auto", Options.Auto, "Ask no questions, use defaults.")
	flag.StringVar(&Options.path, "-path", Options.path, "Directory to install in. Default: current `directory`")
	flag.BoolVar(&Options.noscript, "noscript", Options.noscript, "Skip creating start script. Default: false")
	flag.IntVar(&Options.threads, "threads", Options.threads, "Number of threads to use for downloading. Default: cpucores * 2")
	flag.BoolVar(&Options.integrityUpdate, "integrityUpdate", Options.integrityUpdate, "Whether changed files should be overwritten with fresh copies when updating. Most useful when used with Auto. Default: false")
	flag.BoolVar(&Options.verbose, "verbose", Options.verbose, "Be a bit noisier on actions taken. Default: false")
	flag.BoolVar(&Options.integrity, "integrity", Options.integrity, "Do a full integrity check. integrityUpdate assumed. Default: false")


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
	filename = "serverinstall_79_209"

	Options.Auto = false
	Options.Path = ""
	Options.Noscript = false
	Options.Threads = runtime.NumCPU() * 2
	Options.Integrityupdate = false
	Options.Verbose = false
	Options.Integrity = true
	Options.Latest = false

	Options.Help = false

	parsed := make(map[string]string)

	var flag string

	packIdFound := -1
	versionFound := -1

	for i, arg := range os.Args {
		if i == 0 {
			continue
		}
		if i == 1 {
			tempPack, err := strconv.Atoi(arg)
			if err == nil {
				packIdFound = tempPack
				versionFound = -2
			}
		}
		if i == 2 {
			if packIdFound > 0 {
				tempVer, err := strconv.Atoi(arg)
				if err == nil {
					versionFound = tempVer
				}
			}
		}
		if strings.HasPrefix(arg, "--") {
			if len(flag) > 0 {
				parsed[flag] = "true"
				flag = ""
				flag = arg[2:]
			} else {
				flag = arg[2:]
			}
			continue
		} else {
			if len(flag) > 0 {
				parsed[flag] = arg
				flag = ""
			}
		}
	}

	if len(flag) > 0 {
		parsed[flag] = "true"
		flag = ""
	}

	for name, val := range parsed {
		v := reflect.ValueOf(&Options).Elem().FieldByName(strings.Title(name))
		if v.IsValid() {
			fieldType := v.Type().String()
			switch fieldType {
			case "bool":
				v.SetBool(val == "true")
				break
			case "string":
				log.Println("Setting " + name + " to " + val)
				v.SetString(val)
				break
			case "int64":
				val, ok := strconv.ParseInt(val, 10, 64)
				if ok != nil {
					v.SetInt(val)
				}
			}
		}
	}

	if Options.Help == true {
		PrintUsage()
		os.Exit(0)
	}

	if Options.Latest {
		versionFound = -2
	}

	HandleLaunch(filename, packIdFound, versionFound)
}

func PrintUsage() {
	println("                      _                  _              _     ")
	println("                     | |                | |            | |    ")
	println("  _ __ ___   ___   __| |_ __   __ _  ___| | _____   ___| |__  ")
	println(" | '_ ` _ \\ / _ \\ / _` | '_ \\ / _` |/ __| |/ / __| / __| '_ \\ ")
	println(" | | | | | | (_) | (_| | |_) | (_| | (__|   <\\__ \\| (__| | | |")
	println(" |_| |_| |_|\\___/ \\__,_| .__/ \\__,_|\\___|_|\\_\\___(_)___|_| |_|")
	println("                       | |                                    ")
	println("                       |_|                                    ")
	println(" modpacks.ch server downloader golang - build "+verStr)
	println("Usage:")


	t := reflect.ValueOf(Options)
	for i := 0; i < t.NumField(); i++ {
		name := strings.ToLower(t.Type().Field(i).Name)
		println("--" + name, "-", t.Type().Field(i).Tag.Get("help"))
	}
}

/*func Search(term string) []Modpack {
	termSafe := url.QueryEscape(term)
	result := SearchResult{}
	APICall(SearchURL + termSafe, *result)
	return []Modpack{}
}*/

func HandleLaunch(file string, found int, versionFound int) {
	err, modpackId, versionId := ParseFilename(file)
	if err != nil {
		if found == -1 {
			log.Fatalf("Cannot locate modpack via filename: %v", err)
		}
	}

	if found > -1 {
		modpackId = found
	}

	if versionFound > 0 || versionFound == -2 {
		versionId = versionFound
	}

	var installPath = Options.Path
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
		LogIfVerbose("Making folder %s\n", installPath)
		if err := os.MkdirAll(installPath, os.FileMode(755)); err != nil {
			log.Fatalf("An error occured whilst creating the folder %s: %v", installPath, err)
		}
	} else {
		if !QuestionYN(true,"Path %s already exists - still want to install?", installPath) {
			log.Fatalf("Aborted by user")
		}

	}
	upgrade := false
	if _, err := os.Stat(path.Join(installPath, "version.json")); !os.IsNotExist(err) {
		upgrade = true
	}

	err, modpack := GetModpack(modpackId)
	if err != nil {
		log.Fatalf("Error fetching modpack: %v", err)
	}

	err, versionInfo := modpack.GetVersion(versionId)
	if err != nil {
		log.Fatalf("Error fetching modpack: %v", err)
	}

	downloads := versionInfo.GetDownloads()

	upgradeStr := ""

	if (upgrade) {
		upgradeStr = " as an update"
	}

	if !QuestionYN(true, "Continuing will install %s version %s%s. Do you wish to continue?", modpack.Name, versionInfo.Name, upgradeStr) {
		log.Fatalf("Aborted by user")
	}

	if upgrade {
		err, info := GetVersionInfoFromFile(path.Join(installPath, "version.json"))
		if err != nil {
			if !QuestionYN(true, "An error occurred whilst trying to read the previous installation at %s: %v\nWould you like to continue anyway? You should probably delete folders with mods and configs in it, first!", installPath, err) {
				log.Fatalf("Aborting due to corrupted previous installation")
			} else {
				// TODO: handle removing folders here
			}
		}

		if info.ParentId != modpack.ID {
			if !QuestionYN(true, "Previous modpack is different to this modpack\nWould you like to continue anyway? You should probably delete folders with mods and configs in it, first!", installPath, err) {
				log.Fatalf("Aborting due to different modpack already installed")
			}
		}

		oldDownloads := info.GetDownloads()
		getSortFunc := func(arr []Download) func(i int, j int) bool {
			return func(i int, j int) bool {
				return arr[i].FullPath < arr[j].FullPath
			}
		}

		sort.SliceStable(oldDownloads, getSortFunc(oldDownloads))
		sort.SliceStable(downloads, getSortFunc(downloads))

		lastFound := -1

		downloadsLen := len(downloads)

		var changedFilesOld []Download
		var changedFilesNew []Download

		var newFiles []Download
		var oldDeletedFiles []Download
		var integrityFailures []Download

		for _, oldDown := range oldDownloads {
			for i := lastFound + 1; i < downloadsLen; i++ {
				newDown := downloads[i]
				if oldDown.FullPath == newDown.FullPath {
					lastFound = i
					if oldDown.SHA1 != newDown.SHA1 {
						changedFilesOld = append(changedFilesOld, oldDown)
						changedFilesNew = append(changedFilesNew, newDown)
						LogIfVerbose("Found changed file %s\n", newDown.FullPath)
					} else if Options.Integrity {
						LogIfVerbose("Checking integrity of file %s\n", newDown.FullPath)
						if oldDown.VerifyChecksum(installPath) {
							integrityFailures = append(integrityFailures, oldDown)
						}
					}
					break
				}
				if newDown.FullPath > oldDown.FullPath {
					lastFound = i - 1
					oldDeletedFiles = append(oldDeletedFiles, oldDown)
					LogIfVerbose("Found deleted file %s\n", newDown.FullPath)
					break
				}
				newFiles = append(newFiles, newDown)
				LogIfVerbose("Found new file %s\n", newDown.FullPath)
			}
		}

		log.Printf("This install has %v files changed, %v new files and %v deleted files\n", len(changedFilesOld), len(newFiles), len(oldDeletedFiles))

		var failedChecksums []Download

		for _, oldDown := range changedFilesOld {
			if !oldDown.VerifyChecksum(installPath) {
				failedChecksums = append(failedChecksums, oldDown)
				LogIfVerbose("Detected failed checksum on %s\n", oldDown.FullPath)
			}
		}

		if len(failedChecksums) > 0 {
			overwrite := QuestionYN(Options.Integrityupdate || Options.Integrity, "There are %v failed checksums on files to be updated. This may be as a result of manual config changes. Do you wish to overwrite them with the files from the update?", failedChecksums)
			if overwrite {
				for i := range failedChecksums {
					changedFilesNew[i] = changedFilesNew[len(changedFilesNew)-1]
				}
				changedFilesNew = changedFilesNew[:len(changedFilesNew)-len(failedChecksums)]
			}
		}

		if len(integrityFailures) > 0 {
			if len(failedChecksums) > 0 {
				overwrite := QuestionYN(true, "There are %v failed checksums on already existing files. This may be as a result of manual config changes. Do you wish to overwrite them with the files from the update?", failedChecksums)
				if overwrite {
					changedFilesNew = append(changedFilesNew, integrityFailures...)
				}
			}
		}

		downloads = append(changedFilesNew, newFiles...)

		log.Println("Deleting removed files...")
		for _, down := range oldDeletedFiles {
			filePath := path.Join(installPath, down.FullPath)
			LogIfVerbose("Removing %s\n", filePath)
			if os.Remove(filePath) != nil {
				log.Println("Error occurred whilst removing file " + filePath)
				continue
			}
			tempPath := path.Join(installPath, down.Path)
			dir, err := os.Open(tempPath)
			empty := false
			if err == nil {
				empty = true
				names, _ := dir.Readdirnames(-1)
				for _, name := range names {
					if name != "." && name != ".." {
						empty = false
						break
					}
				}
			}
			if empty {
				LogIfVerbose("Removing %s as is empty\n", tempPath)
				if os.RemoveAll(tempPath) != nil {
					log.Println("Error occurred whilst removing folder " + tempPath)
				}
			}
		}

		log.Println("Performing update...")
	} else {
		log.Println("Performing installation...")
	}

	err, ml := versionInfo.GetModLoader()

	if err != nil {
		log.Fatalf("Error getting Modloader: %v", err)
	}

	modLoaderDls := ml.GetDownloads(installPath)

	downloads = append(downloads, modLoaderDls...)

	grabs, err := GetBatch(Options.Threads, installPath, downloads...)
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

	versionInfo.WriteJson(installPath)

	if !Options.Noscript {
		versionInfo.WriteStartScript(installPath, ml)
	}

	// return the number of failed downloads as exit code
	os.Exit(failed)
}

func ParseFilename(file string) (error, int, int) {
	re := regexp.MustCompile("^" + BaseName + "_(\\d+)_(\\d+)")
	matched := re.FindStringSubmatch(file)
	if len(matched) < 3 {
		return errors.New("unable to parse filename: " + file), -1, -1
	}
	modpackId, err := strconv.Atoi(matched[1])
	if err != nil {
		return errors.New("unable to parse filename: " + file), -1, -1
	}
	versionId, err := strconv.Atoi(matched[2])
	if err != nil {
		return errors.New("unable to parse filename: " + file), -1, -1
	}
	return nil, modpackId, versionId
}

func (v VersionInfo) GetModLoader() (error, ModLoader) {
	var ret ModLoader
	var modLoader Target
	var minecraftTar Target

	for _, target := range v.Targets {
		if target.Type == "modloader" {
			modLoader = target
		}
		if target.Type == "game" {
			minecraftTar = target
		}
	}
	mc := Minecraft{}
	mc.RawVersion = minecraftTar.Version
	if err := mc.Parse(); err != nil {
		log.Fatalf("Error parsing Version: %v", err)
	}

	if modLoader.Name == "forge" {
		return GetForge(modLoader, mc)
	}
	return errors.New(fmt.Sprintf("Unable to locate Mod Loader for %s %s %s", modLoader.Name, modLoader.Version, mc.RawVersion)), ret
}

func APICall(url string, val interface{}) error {
	println(url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	stringRet, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	resp.Body.Close()

	err = json.Unmarshal(stringRet, &val)

	if err != nil {
		return err
	}

	return nil
}

func LogIfVerbose(fmt string, v ...interface{}) {
	if Options.Verbose {
		log.Printf(fmt, v...)
	}
}

func GetBatch(workers int, dst string, downloads ...Download) (<-chan *grab.Response, error) {
	fi, err := os.Stat(dst)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("destination is not a directory")
	}

	reqs := make([]*grab.Request, len(downloads))
	for i := 0; i < len(downloads); i++ {
		req, err := grab.NewRequest(path.Join(dst, downloads[i].Path, downloads[i].Name), downloads[i].URL.String())
		if err != nil {
			return nil, err
		}
		req.NoResume = true // force re-download
		if len(downloads[i].SHA1) > 0 {
			byteHex, _ := hex.DecodeString(downloads[i].SHA1)
			req.SetChecksum(crypto.SHA1.New(), byteHex, false)
		}

		reqs[i] = req
	}

	ch := grab.DefaultClient.DoBatch(workers, reqs...)
	return ch, nil
}

func updateUI(responses []*grab.Response) {
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
				fmt.Printf("Finished %s\n",
					resp.Filename,
					)
			}
			responses[i] = nil
		}
	}
}