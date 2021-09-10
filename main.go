// This package's main function copies files from one folder to another but doesn't override files that are the same (files are the same if they have the same size)
package main

import (
	"fmt"
	"log"
	"mirror/mirror"
	"os"
)

const (
	MsgCanceling   = "canceling"
	MsgCalculating = "gathering info about files"
	MgsAreYouSure  = "Do you want to continue?"
	MsgLogging     = "Also a log file named 'log' will be generated."
	MsgNothingToDo = "there is nothing to do"
	MsgErrOccurred = "an error occurred"
	MsgFinished    = "the program finished successfully"
	MsgDone        = "done"
)

func main() {
	dst, src, cleaningMode, err := mirror.VetFlags()
	checkErr(err)

	if cleaningMode {
		doCleaning(dst, src)
	} else {
		doCopying(dst, src)
	}

	log.Println(MsgFinished)
}

func doCopying(dst, src string) {
	if !mirror.AskQuestion(fmt.Sprintf("files from %q will be copied to %q. %s", src, dst, MgsAreYouSure)) {
		exitWithZero(MsgCanceling)
	}

	missingFolders, missingFiles, totalSize := srcDstDiff(dst, src, false)

	if !mirror.AskQuestion(fmt.Sprintf("%d files will be coppied (%s MB) and %d folders will be created. %s %s", len(missingFiles), mirror.BytesToMB(totalSize), len(missingFolders), MsgLogging, MgsAreYouSure)) {
		exitWithZero(MsgCanceling)
	}

	err := mirror.TruncateLogFile()
	checkErr(err)

	if len(missingFolders) > 0 {
		err = mirror.MakeFolders(missingFolders, dst)
		checkErr(err)
		log.Println(MsgDone)
	}

	if len(missingFiles) > 0 {
		err = mirror.CopyFiles(missingFiles, totalSize, src, dst)
		checkErr(err)
		log.Println(MsgDone)
	}
}

func doCleaning(dst, src string) {
	if !mirror.AskQuestion(fmt.Sprintf("files may be deleted in the %q folder. %s", dst, MgsAreYouSure)) {
		exitWithZero(MsgCanceling)
	}

	foldersToClean, filesToClean, totalSize := srcDstDiff(dst, src, true)

	if !mirror.AskQuestion(fmt.Sprintf("%d files (%s MB) and %d folders will be deleted. %s %s", len(filesToClean), mirror.BytesToMB(totalSize), len(foldersToClean), MsgLogging, MgsAreYouSure)) {
		exitWithZero(MsgCanceling)
	}

	err := mirror.TruncateLogFile()
	checkErr(err)

	if len(filesToClean) > 0 {
		err = mirror.CleanFiles(filesToClean, totalSize, dst)
		checkErr(err)
		log.Println(MsgDone)
	}

	if len(foldersToClean) > 0 {
		err = mirror.CleanFolders(foldersToClean, dst)
		checkErr(err)
		log.Println(MsgDone)
	}
}

func srcDstDiff(dst, src string, cleaningMod bool) (folders mirror.Folder, files mirror.File, totalSize int64) {
	log.Println(MsgCalculating)

	srcFolders, srcFiles, err := mirror.ReadFolder(src)
	checkErr(err)

	dstFolders, dstFiles, err := mirror.ReadFolder(dst)
	checkErr(err)

	if cleaningMod {
		folders = mirror.FoldersToClean(dstFolders, srcFolders)
		files, totalSize = mirror.FilesToClean(dstFiles, srcFiles)
	} else {
		folders = mirror.MissingFolders(dstFolders, srcFolders)
		files, totalSize = mirror.MissingFiles(dstFiles, srcFiles)
	}

	if len(files) == 0 && len(folders) == 0 {
		exitWithZero(MsgNothingToDo)
	}

	return
}

func checkErr(err error) {
	if err != nil {
		log.Fatalln(MsgErrOccurred, err)
	}
}

func exitWithZero(msg string) {
	log.Println(msg)
	os.Exit(0)
}
