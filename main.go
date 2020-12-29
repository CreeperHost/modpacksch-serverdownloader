package main

import (
	"crypto"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/cavaliercoder/grab"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
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
)

var Options struct {
	auto bool
	path string
	startScript bool
	threads int
	integrityUpdate bool
	integrity bool
	verbose bool

	help bool
}


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

	Options.auto = false
	Options.path = ""
	Options.startScript = false
	Options.threads = runtime.NumCPU() * 2
	Options.integrityUpdate = false
	Options.verbose = false
	Options.integrity = true

	Options.help = false


	flag.BoolVar(&Options.auto, "auto", Options.auto, "Ask no questions, use defaults.")
	flag.StringVar(&Options.path, "path", Options.path, "Directory to install in. Default: current `directory`")
	flag.BoolVar(&Options.startScript, "startScript", Options.startScript, "Generate a start script using specifications. Default: false")
	flag.IntVar(&Options.threads, "threads", Options.threads, "Number of threads to use for downloading. Default: cpucores * 2")
	flag.BoolVar(&Options.integrityUpdate, "integrityUpdate", Options.integrityUpdate, "Whether changed files should be overwritten with fresh copies when updating. Most useful when used with auto. Default: false")
	flag.BoolVar(&Options.verbose, "verbose", Options.verbose, "Be a bit noisier on actions taken. Default: false")
	flag.BoolVar(&Options.integrity, "integrity", Options.integrity, "Do a full integrity check. integrityUpdate assumed. Default: false")

	flag.BoolVar(&Options.help,"help", Options.help, "This help screen")
	flag.Parse()

	if Options.help {
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
	var installPath = Options.path
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
			LogIfVerbose("Making folder %s\n", installPath)
			if err := os.MkdirAll(installPath, os.FileMode(755)); err != nil {
				log.Fatalf("An error occured whilst creating the folder %s: %v", installPath, err)
			}
		}
	}
	upgrade := false
	if _, err := os.Stat(path.Join(installPath, "version.json")); !os.IsNotExist(err) {
		upgrade = true
	}

	err, modpackId, versionId := ParseFilename(file)

	if err != nil {
		log.Fatalf("Cannot locate modpack via filename %v", err)
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
		upgradeStr = "as an upgrade"
	}

	if !QuestionYN(true, "Continuing will install %s version %s%s. Do you wish to continue?", modpack.Name, versionInfo.Name, upgradeStr) {
		log.Fatalf("Exiting on user input")
	}

	if upgrade {
		err, info := GetVersionInfoFromFile(path.Join(installPath, "version.json"))
		if err != nil {
			if !QuestionYN(true, "An error occurred whilst trying to read the previous installation at %s: %v\nWould you like to continue anyway? Any folders which exist in the update will be deleted completely.", installPath, err) {
				log.Fatalf("Aborting due to corrupted previous installation")
			} else {
				// TODO: handle removing folders here
			}
		}

		if info.ParentId != modpack.ID {
			if !QuestionYN(true, "Previous modpack is different to this modpack\nWould you like to continue anyway?", installPath, err) {
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
					} else if Options.integrity {
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
			overwrite := QuestionYN(Options.integrityUpdate || Options.integrity, "There are %v failed checksums on files to be updated. This may be as a result of manual config changes. Do you wish to overwrite them with the files from the update?", failedChecksums)
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

	grabs, err := GetBatch(Options.threads, installPath, downloads...)
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

	// return the number of failed downloads as exit code
	os.Exit(failed)
}

func ParseFilename(file string) (error, int, int) {
	re := regexp.MustCompile("^serverinstall_(\\d+)_(\\d+)")
	matched := re.FindStringSubmatch(file)
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
	if Options.verbose {
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