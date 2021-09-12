// Package mirror contains functions that are used by the main package
package mirror

import (
	"bufio"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

const (
	ErrWrongArgs               = CustomErr("wrong arguments, use the -h flag for help")
	ErrSrcNotFound             = CustomErr("source folder doesn't exist")
	ErrDstNotFound             = CustomErr("destination folder doesn't exist")
	ErrOnlyFoldersOrFiles      = CustomErr("the function accepts only folders and files")
	FolderToIgnore             = "dont_mirror"
	LogFile                    = "log"
	ZeroPercent                = "0%"
	BytesInMB                  = 1e6
	FolderPerm                 = 0755
	FilePerm                   = 0644
	LogMadeFolders             = "directories made: (if a folder had some parent directories, they were also created)"
	LogCleanedFolders          = "directories removed: (if a folder had some subdirectories, they were also removed)"
	LogCopiedFiles             = "files copied:"
	LogCleanedFiles            = "files removed:"
	MsgProgressCopyingFiles    = "copying files:"
	MsgProgressMakingFolders   = "making folders:"
	MsgProgressCleaningFiles   = "removing files:"
	MsgProgressCleaningFolders = "removing folders:"
	FlagNameSrc                = "src"
	FlagNameDst                = "dst"
	FlagNameC                  = "c"
	FlagUsageSrc               = "source folder"
	FlagUsageDst               = "destination folder"
	FlagUsageC                 = "cleaning mode"
)

type (
	CustomErr string
	Folder    map[string]struct{}
	File      map[string]int64
)

func (e CustomErr) Error() string {
	return string(e)
}

// AskQuestion prints question and returns true if it gets y/Y on input
func AskQuestion(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	log.Printf("%s (y/n)\n", question)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if !strings.EqualFold("y", answer) {
		return false
	}
	return true
}

// VetFlags checks if flags are valid and rewrites them into an absolute path
func VetFlags() (dst, src string, cleaningMode bool, err error) {
	srcPath := flag.String(FlagNameSrc, "", FlagUsageSrc)
	dstPath := flag.String(FlagNameDst, "", FlagUsageDst)
	cFlag := flag.Bool(FlagNameC, false, FlagUsageC)

	flag.Parse()

	src = *srcPath
	dst = *dstPath

	if src == "" || dst == "" {
		err = ErrWrongArgs
		return
	}

	dst, err = filepath.Abs(*dstPath)
	if err != nil {
		return
	}

	src, err = filepath.Abs(*srcPath)
	if err != nil {
		return
	}

	if f, errF := os.Stat(src); os.IsNotExist(errF) || !f.IsDir() {
		err = ErrSrcNotFound
		return
	}

	if f, errF := os.Stat(dst); os.IsNotExist(errF) || !f.IsDir() {
		err = ErrDstNotFound
		return
	}

	if *cFlag {
		cleaningMode = true
	}

	return
}

// ReadFolder returns paths of folders and files. The paths are relative to the path that was passed as an argument
func ReadFolder(path string) (folders Folder, files File, err error) {
	folders = make(Folder)
	files = make(File)
	err = readFolder(path, path, folders, files)
	return
}

func readFolder(path string, startingPath string, folders Folder, files File) error {
	items, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, item := range items {
		currentName := item.Name()
		currentPath := filepath.Join(path, currentName)
		currentTrimmedPath, err := filepath.Rel(startingPath, currentPath)
		if err != nil {
			return err
		}

		if item.IsDir() {
			if currentName != FolderToIgnore {
				folders[currentTrimmedPath] = struct{}{}
				if err = readFolder(currentPath, startingPath, folders, files); err != nil {
					return err
				}
			}
		} else {
			info, err := item.Info()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != os.ModeSymlink {
				files[currentTrimmedPath] = info.Size()
			}
		}
	}
	return nil
}

// MissingFolders returns directories that are present in src but not in dst
func MissingFolders(dst, src Folder) Folder {
	res := make(Folder)

	for folder, v := range src {
		if _, ok := dst[folder]; !ok {
			res[folder] = v
		}
	}
	return res
}

// FoldersToClean returns directories that are present in dst but not in src
func FoldersToClean(dst, src Folder) Folder {
	res := make(Folder)

	for folder, v := range dst {
		if _, ok := src[folder]; !ok {
			res[folder] = v
		}
	}
	return res
}

// MissingFiles returns files that are present in src but not in dst or have different size
func MissingFiles(dst, src File) (res File, totalSize int64) {
	res = make(File)

	for file, size := range src {
		if _size, ok := dst[file]; !ok || _size != size {
			res[file] = size
			totalSize += size
		}
	}
	return
}

// FilesToClean returns files that are present in dst but not in src
func FilesToClean(dst, src File) (res File, totalSize int64) {
	res = make(File)

	for file, size := range dst {
		if _, ok := src[file]; !ok {
			res[file] = size
			totalSize += size
		}
	}
	return
}

// MakeFolders makes directories with os.MkdirAll in path directory and logs progress
func MakeFolders(folders Folder, path string) error {
	var recentlyLoggedProgress, counter int

	f, err := initLogFile()
	if err != nil {
		return err
	}

	LogToFile(f, LogMadeFolders+"\n")
	log.Println(MsgProgressMakingFolders, ZeroPercent)

	sortedFolders := keepFoldersWithLongestPrefix(folders)
	for _, folder := range sortedFolders {
		if err = os.MkdirAll(filepath.Join(path, folder), FolderPerm); err != nil {
			return err
		}

		logProgressFolders(&recentlyLoggedProgress, &counter, len(sortedFolders), MsgProgressMakingFolders)

		LogToFile(f, folder)
	}

	err = f.Close()
	return err
}

// CleanFolders removes directories with os.RemoveAll in path directory and logs progress
func CleanFolders(folders Folder, path string) error {
	var recentlyLoggedProgress, counter int

	f, err := initLogFile()
	if err != nil {
		return err
	}

	LogToFile(f, LogCleanedFolders+"\n")
	log.Println(MsgProgressCleaningFolders, ZeroPercent)

	sortedFolders := keepFoldersWithShortestPrefix(folders)
	for _, folder := range sortedFolders {
		if err = os.RemoveAll(filepath.Join(path, folder)); err != nil {
			return err
		}

		logProgressFolders(&recentlyLoggedProgress, &counter, len(sortedFolders), MsgProgressCleaningFolders)

		LogToFile(f, folder)
	}

	err = f.Close()
	return err
}

// CopyFiles copies files and logs progress. The 'files' parameter should contain relative paths
func CopyFiles(files File, totalSize int64, src, dst string) error {
	var bytesWritten, recentlyLoggedProgress int64

	l, err := initLogFile()
	if err != nil {
		return err
	}

	LogToFile(l, LogCopiedFiles+"\n")
	log.Println(MsgProgressCopyingFiles, ZeroPercent)

	for _, file := range sortFoldersOrFiles(files) {
		s, err := os.Open(filepath.Join(src, file))
		if err != nil {
			return err
		}

		d, err := os.Create(filepath.Join(dst, file))
		if err != nil {
			return err
		}

		written, err := io.Copy(d, s)
		if err != nil {
			return err
		}
		bytesWritten += written

		if err = s.Close(); err != nil {
			return err
		}
		if err = d.Close(); err != nil {
			return err
		}

		logProgressFiles(&recentlyLoggedProgress, totalSize, bytesWritten, MsgProgressCopyingFiles)

		LogToFile(l, file)
	}

	if err = l.Close(); err != nil {
		return err
	}

	return nil
}

// CleanFiles removes files and logs progress. The 'files' parameter should contain relative paths
func CleanFiles(files File, totalSize int64, path string) error {
	var bytesDeleted, recentlyLoggedProgress int64

	l, err := initLogFile()
	if err != nil {
		return err
	}

	LogToFile(l, LogCleanedFiles+"\n")
	log.Println(MsgProgressCleaningFiles, ZeroPercent)

	for _, file := range sortFoldersOrFiles(files) {
		info, err := os.Stat(filepath.Join(path, file))
		if err != nil {
			return err
		}
		bytesDeleted += info.Size()

		if err = os.Remove(filepath.Join(path, file)); err != nil {
			return err
		}

		logProgressFiles(&recentlyLoggedProgress, totalSize, bytesDeleted, MsgProgressCleaningFiles)

		LogToFile(l, file)
	}

	if err = l.Close(); err != nil {
		return err
	}

	return nil
}

// ThousandSeparator adds space after each thousand: 1000000 -> 1 000 000
func ThousandSeparator(n string) string {
	if len(n) < 4 {
		return n
	}
	return ThousandSeparator(n[:len(n)-3]) + " " + n[len(n)-3:]
}

// WriteNewLineIfNotEmpty writes a new line into a file if it's not empty
func WriteNewLineIfNotEmpty(f *os.File) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() != 0 {
		if _, err = f.WriteString("\n"); err != nil {
			return err
		}
	}
	return nil
}

func LogToFile(w io.Writer, message string) {
	log.SetOutput(w)
	log.Println(message)
	log.SetOutput(os.Stdout)
}

func TruncateLogFile() error {
	err := os.WriteFile(LogFile, nil, FilePerm)
	return err
}

func BytesToMB(size int64) string {
	return ThousandSeparator(strconv.FormatInt(size/BytesInMB, 10))
}

func logProgressFiles(recentlyLoggedProgress *int64, totalSize, bytesWritten int64, msg string) {
	// howOftenToLog and progress are in percentage
	var howOftenToLog, progress int64 = 10, 100

	if totalSize > 0 {
		progress = bytesWritten * 100 / totalSize
	} else if totalSize == 0 && *recentlyLoggedProgress > 0 {
		return
	}

	if progress >= *recentlyLoggedProgress+howOftenToLog {
		log.Printf("%s %d%%\n", msg, progress)
		*recentlyLoggedProgress += progress
	}
}

func logProgressFolders(recentlyLoggedProgress, counter *int, foldersLen int, msg string) {
	*counter++
	// howOftenToLog and progress are in percentage
	var howOftenToLog, progress = 10, *counter * 100 / foldersLen

	if progress >= *recentlyLoggedProgress+howOftenToLog {
		log.Printf("%s %d%%\n", msg, progress)
		*recentlyLoggedProgress += progress
	}
	return
}

func sortFoldersOrFiles(m interface{}) []string {
	_, isFolder := m.(Folder)
	_, isFile := m.(File)

	if !isFolder && !isFile {
		panic(ErrOnlyFoldersOrFiles)
	}

	res := make([]string, 0, reflect.ValueOf(m).Len())
	for _, v := range reflect.ValueOf(m).MapKeys() {
		res = append(res, v.String())
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i] < res[j]
	})
	return res
}

func initLogFile() (logFile *os.File, err error) {
	logFile, err = os.OpenFile(LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, FilePerm)
	if err != nil {
		return
	}
	err = WriteNewLineIfNotEmpty(logFile)
	return
}

func keepFoldersWithLongestPrefix(folders Folder) (res []string) {
	sorted := sortFoldersOrFiles(folders)

	for i := 0; i < len(folders)-1; i++ {
		if filepath.Dir(sorted[i+1]) == filepath.Dir(sorted[i]) || !strings.HasPrefix(sorted[i+1], sorted[i]) {
			res = append(res, sorted[i])
		}
	}
	res = append(res, sorted[len(folders)-1])

	return
}

func keepFoldersWithShortestPrefix(folders Folder) (res []string) {
	sorted := sortFoldersOrFiles(folders)

	for i := len(folders) - 1; i > 0; i-- {
		if filepath.Dir(sorted[i]) == filepath.Dir(sorted[i-1]) || !strings.HasPrefix(sorted[i], sorted[i-1]) {
			res = append(res, sorted[i])
		}
	}
	res = append(res, sorted[0])

	return
}
