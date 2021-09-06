// The tests are kinda wonky since I was too lazy to mock the file system, so the tests use the "real" file system instead
package mirror

import (
	"errors"
	"flag"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

var (
	srcFolders, dstFolders, missingFolders, foldersToClean Folder
	srcFiles, dstFiles, missingFiles, filesToClean         File
	sizeOfMissingFiles, sizeOfFilesToClean                 int64
)

const (
	srcPathTest = "src"
	dstPathTest = "dst"
)

func TestVetFlags(t *testing.T) {
	makeTestFolders(t)

	t.Run("with correct flags", func(t *testing.T) {
		setFlags(t, dstPathTest, srcPathTest, false)
		_, src, _, err := VetFlags()
		assertError(t, nil, err)
		wantSrc, err := filepath.Abs(srcPathTest)
		assertError(t, nil, err)
		assert(t, wantSrc, src)
	})

	t.Run("with incorrect flags", func(t *testing.T) {
		setFlags(t, "aaa", srcPathTest, false)
		_, _, _, err := VetFlags()
		assertError(t, ErrDstNotFound, err)
	})

	t.Run("with empty flags", func(t *testing.T) {
		setFlags(t, "", "", false)
		_, _, _, err := VetFlags()
		assertError(t, ErrWrongArgs, err)
	})

	cleanTestFolders(t)
}

func TestReadFolder(t *testing.T) {
	makeTestFolders(t)

	t.Run("with correct path", func(t *testing.T) {
		gotFolders, gotFiles, err := ReadFolder(srcPathTest)
		assert(t, srcFolders, gotFolders)
		assert(t, srcFiles, gotFiles)
		assertError(t, nil, err)
	})

	t.Run("with incorrect path", func(t *testing.T) {
		_, _, err := ReadFolder("aaa")
		if _, ok := err.(*fs.PathError); !ok {
			t.Errorf("wanted *fs.PathError, but got %q", err)
		}
	})

	cleanTestFolders(t)
}

func TestMissingFolders(t *testing.T) {
	makeTestFolders(t)

	got := MissingFolders(dstFolders, srcFolders)
	assert(t, missingFolders, got)

	cleanTestFolders(t)
}

func TestFoldersToClean(t *testing.T) {
	makeTestFolders(t)

	got := FoldersToClean(dstFolders, srcFolders)
	assert(t, foldersToClean, got)

	cleanTestFolders(t)
}

func TestMissingFiles(t *testing.T) {
	makeTestFolders(t)

	got, size := MissingFiles(dstFiles, srcFiles)
	assert(t, missingFiles, got)
	assert(t, sizeOfMissingFiles, size)

	cleanTestFolders(t)
}

func TestFilesToClean(t *testing.T) {
	makeTestFolders(t)

	got, size := FilesToClean(dstFiles, srcFiles)
	assert(t, filesToClean, got)
	assert(t, sizeOfFilesToClean, size)

	cleanTestFolders(t)
}

func TestMakeFolders(t *testing.T) {
	makeTestFolders(t)

	err := MakeFolders(missingFolders, dstPathTest)
	assertError(t, nil, err)

	src, _, err := ReadFolder(srcPathTest)
	assertError(t, nil, err)

	dst, _, err := ReadFolder(dstPathTest)
	assertError(t, nil, err)

	missing := MissingFolders(dst, src)

	assert(t, len(missing), 0)

	cleanTestFolders(t)
}

func TestCleanFolders(t *testing.T) {
	makeTestFolders(t)

	err := CleanFolders(missingFolders, srcPathTest)
	assertError(t, nil, err)

	err = CleanFolders(foldersToClean, dstPathTest)
	assertError(t, nil, err)

	src, _, err := ReadFolder(srcPathTest)
	assertError(t, nil, err)

	dst, _, err := ReadFolder(dstPathTest)
	assertError(t, nil, err)

	assert(t, dst, src)

	cleanTestFolders(t)
}

func TestCopyFiles(t *testing.T) {
	makeTestFolders(t)

	err := MakeFolders(missingFolders, dstPathTest)
	assertError(t, nil, err)

	err = CopyFiles(missingFiles, sizeOfMissingFiles, srcPathTest, dstPathTest)
	assertError(t, nil, err)

	err = CleanFiles(filesToClean, dstPathTest)
	assertError(t, nil, err)

	_, src, err := ReadFolder(srcPathTest)
	assertError(t, nil, err)

	_, dst, err := ReadFolder(dstPathTest)
	assertError(t, nil, err)

	assert(t, src, dst)

	cleanTestFolders(t)
}

func TestCleanFiles(t *testing.T) {
	makeTestFolders(t)

	err := CopyFiles(missingFiles, sizeOfMissingFiles, srcPathTest, dstPathTest)
	assertError(t, nil, err)

	err = CleanFiles(filesToClean, dstPathTest)
	assertError(t, nil, err)

	_, src, err := ReadFolder(srcPathTest)
	assertError(t, nil, err)

	_, dst, err := ReadFolder(dstPathTest)
	assertError(t, nil, err)

	assert(t, dst, src)

	cleanTestFolders(t)
}

func TestWriteNewLineIfNotEmpty(t *testing.T) {
	fileName := "f"

	f, err := os.Create(fileName)
	assertError(t, nil, err)

	err = WriteNewLineIfNotEmpty(f)
	assertError(t, nil, err)

	_, err = f.WriteString("aaa")
	assertError(t, nil, err)

	err = WriteNewLineIfNotEmpty(f)
	assertError(t, nil, err)

	err = f.Close()
	assertError(t, nil, err)

	dat, err := os.ReadFile(fileName)
	assertError(t, nil, err)

	err = os.Remove(fileName)
	assertError(t, nil, err)

	assert(t, "aaa\n", string(dat))
}

func TestThousandSeparator(t *testing.T) {
	tests := []struct {
		name, input, expected string
	}{
		{name: "no apostrophe", input: "100", expected: "100"},
		{name: "one apostrophe", input: "1000", expected: "1'000"},
		{name: "three apostrophes", input: "10000000000", expected: "10'000'000'000"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ThousandSeparator(test.input)
			if got != test.expected {
				t.Errorf("input %q, got %q, expexted %q", test.input, got, test.expected)
			}
		})
	}
}

func makeTestFolders(t testing.TB) {
	t.Helper()

	makeTestSrcFolder(t)
	makeTestDstFolder(t)
}

func cleanTestFolders(t testing.TB) {
	t.Helper()

	err := os.RemoveAll(srcPathTest)
	assertError(t, nil, err)
	err = os.RemoveAll(dstPathTest)
	assertError(t, nil, err)
}

func makeTestSrcFolder(t testing.TB) {
	t.Helper()

	// make folders
	err := os.MkdirAll(mcpp(srcPathTest+"/same_1/same_2/not_in_dst"), FolderPerm)
	assertError(t, nil, err)

	srcFolders = Folder{"same_1": {}, mcpp("same_1/same_2"): {}, mcpp("same_1/same_2/not_in_dst"): {}}
	missingFolders = Folder{mcpp("same_1/same_2/not_in_dst"): {}}

	// make files
	err = ioutil.WriteFile(mcpp(srcPathTest+"/_same_1"), []byte("s"), FilePerm)
	assertError(t, nil, err)

	err = ioutil.WriteFile(mcpp(srcPathTest+"/same_1/same_2/_not_in_dst"), []byte("n"), FilePerm)
	assertError(t, nil, err)

	err = ioutil.WriteFile(mcpp(srcPathTest+"/same_1/_different"), []byte("dd"), FilePerm)
	assertError(t, nil, err)

	srcFiles = File{"_same_1": 1, mcpp("same_1/same_2/_not_in_dst"): 1, mcpp("same_1/_different"): 2}
	missingFiles = File{mcpp("same_1/same_2/_not_in_dst"): 1, mcpp("same_1/_different"): 2}

	sizeOfMissingFiles = 3

	return
}

func makeTestDstFolder(t testing.TB) {
	t.Helper()

	// make folders
	err := os.MkdirAll(mcpp(dstPathTest+"/same_1/same_2/not_in_src"), FolderPerm)
	assertError(t, nil, err)

	dstFolders = Folder{"same_1": {}, mcpp("same_1/same_2"): {}, mcpp("same_1/same_2/not_in_src"): {}}
	foldersToClean = Folder{mcpp("same_1/same_2/not_in_src"): {}}

	// make files
	err = ioutil.WriteFile(mcpp(dstPathTest+"/_same_1"), []byte("s"), FilePerm)
	assertError(t, nil, err)

	err = ioutil.WriteFile(mcpp(dstPathTest+"/same_1/same_2/_not_in_src"), []byte("n"), FilePerm)
	assertError(t, nil, err)

	err = ioutil.WriteFile(mcpp(dstPathTest+"/same_1/_different"), []byte("d"), FilePerm)
	assertError(t, nil, err)

	dstFiles = File{"_same_1": 1, mcpp("same_1/same_2/_not_in_src"): 1, mcpp("same_1/_different"): 1}
	filesToClean = File{mcpp("same_1/same_2/_not_in_src"): 1}

	sizeOfFilesToClean = 1

	return
}

func setFlags(t testing.TB, dst, src string, c bool) {
	t.Helper()

	// reset flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	os.Args = os.Args[:1]
	os.Args = append(os.Args, "-"+FlagNameDst, dst, "-"+FlagNameSrc, src, "-"+FlagNameC, strconv.FormatBool(c))
}

// mcpp makes a cross-platform path from unix path
func mcpp(unixPath string) string {
	pathSplit := strings.Split(unixPath, "/")
	return filepath.Join(pathSplit...)
}

func assert(t testing.TB, want, got interface{}) {
	t.Helper()

	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %#v, got %#v", want, got)
	}
}

func assertError(t testing.TB, want, got error) {
	t.Helper()

	if !errors.Is(want, got) {
		t.Fatalf("wrong error, want %q, got %q", want, got)
	}
}
