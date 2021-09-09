// Package mirror contains functions that are used by the main package
package mirror

import (
	"bufio"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	ErrWrongArgs        = CustomErr("wrong arguments, use the -h flag for help")
	ErrSrcNotFound      = CustomErr("source folder doesn't exist")
	ErrDstNotFound      = CustomErr("destination folder doesn't exist")
	FolderToIgnore      = "dont_mirror"
	LogFile             = "log"
	BytesInMB           = 1e6
	FolderPerm          = 0755
	FilePerm            = 0644
	LogMadeFolders      = "directories made:"
	LogCleanedFolders   = "directories removed:"
	LogCopiedFiles      = "files copied:"
	LogCleanedFiles     = "files removed:"
	MsgDone             = "done"
	MsgCopyProgress     = "copying files:"
	MsgCleaningProgress = "removing files:"
	FlagNameSrc         = "src"
	FlagNameDst         = "dst"
	FlagNameC           = "c"
	FlagUsageSrc        = "source folder"
	FlagUsageDst        = "destination folder"
	FlagUsageC          = "cleaning mode"
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
			files[currentTrimmedPath] = info.Size()
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
	f, err := os.OpenFile(LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, FilePerm)
	if err != nil {
		return err
	}

	if err = WriteNewLineIfNotEmpty(f); err != nil {
		return err
	}
	LogToFile(f, LogMadeFolders+"\n")

	for folder := range folders {
		fullPath := filepath.Join(path, folder)
		if err = os.MkdirAll(fullPath, FolderPerm); err != nil {
			return err
		}
		LogToFile(f, folder)
	}

	err = f.Close()
	return err
}

// CleanFolders removes directories with os.Remove in path directory and logs progress
func CleanFolders(folders Folder, path string) error {
	f, err := os.OpenFile(LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, FilePerm)
	if err != nil {
		return err
	}

	if err = WriteNewLineIfNotEmpty(f); err != nil {
		return err
	}
	LogToFile(f, LogCleanedFolders+"\n")

	var sortedFolders []string
	for folder := range folders {
		sortedFolders = append(sortedFolders, filepath.Join(path, folder))
	}
	sort.Slice(sortedFolders, func(i, j int) bool {
		return sortedFolders[i] > sortedFolders[j]
	})

	for _, folder := range sortedFolders {
		if err = os.Remove(folder); err != nil {
			return err
		}
		LogToFile(f, folder)
	}

	err = f.Close()
	return err
}

// CopyFiles copies files and logs progress. The 'files' parameter should contain relative paths
func CopyFiles(files File, totalSize int64, src, dst string) error {
	var bytesWritten, progressInPercentage int64
	// howOftenToLog is in percentage
	var howOftenToLog int64 = 10

	l, err := os.OpenFile(LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, FilePerm)
	if err != nil {
		return err
	}

	if err = WriteNewLineIfNotEmpty(l); err != nil {
		return err
	}
	LogToFile(l, LogCopiedFiles+"\n")

	for file := range files {
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

		if totalSize > 0 && bytesWritten*100/totalSize > progressInPercentage {
			log.Printf("%s %d%%\n", MsgCopyProgress, progressInPercentage)
			progressInPercentage += howOftenToLog
		}

		LogToFile(l, file)
	}

	if err = l.Close(); err != nil {
		return err
	}

	log.Println(MsgDone)
	return nil
}

// CleanFiles removes files and logs progress. The 'files' parameter should contain relative paths
func CleanFiles(files File, path string) error {
	var progressInPercentage, counter int
	// howOftenToLog is in percentage
	howOftenToLog := 10

	l, err := os.OpenFile(LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, FilePerm)
	if err != nil {
		return err
	}

	if err = WriteNewLineIfNotEmpty(l); err != nil {
		return err
	}
	LogToFile(l, LogCleanedFiles+"\n")

	for file := range files {
		if err = os.Remove(filepath.Join(path, file)); err != nil {
			return err
		}

		counter++

		if counter*100/len(files) > progressInPercentage {
			log.Printf("%s %d%%\n", MsgCleaningProgress, progressInPercentage)
			progressInPercentage += howOftenToLog
		}

		LogToFile(l, file)
	}

	if err = l.Close(); err != nil {
		return err
	}

	log.Println(MsgDone)
	return nil
}

// ThousandSeparator adds apostrophe after each thousand: 1000000 -> 1'000'000
func ThousandSeparator(n string) string {
	if len(n) < 4 {
		return n
	}
	return ThousandSeparator(n[:len(n)-3]) + "'" + n[len(n)-3:]
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
