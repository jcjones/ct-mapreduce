package storage

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// type StorageBackend interface {
// 	Store(id string, b []byte) (int, error)
// 	Load(id string) ([]byte, error)
// 	List(path string, walkFn filepath.WalkFunc) error
// 	// Someday: Add Reader and Writer methods
// }

type BackendTestHarness interface {
	MakeFolder(id string) string
	MakeFile(id string, data []byte)
	Remove(id string)
}

func storeAndLoad(t *testing.T, path string, db StorageBackend, data []byte) {
	err := db.Store(path, data)
	if err != nil {
		t.Fatalf("Should have stored %d bytes: %+v", len(data), err)
	}

	t.Logf("Now loading %s", path)

	loaded, err := db.Load(path)
	if err != nil {
		t.Fatalf("Should have loaded: %+v", err)
	}

	if !bytes.Equal(data, loaded) {
		t.Fatalf("Data should match exactly")
	}
}

func BackendTestStoreLoad(t *testing.T, db StorageBackend, h BackendTestHarness) {
	path := filepath.Join(h.MakeFolder(t.Name()), "test_file")

	storeAndLoad(t, path, db, []byte{})
	storeAndLoad(t, path, db, []byte{0x01})
	storeAndLoad(t, path, db, []byte{0x00, 0x01, 0x02})
	storeAndLoad(t, path, db, make([]byte, 1*1024*1024))
	// storeAndLoad(t, path, db, make([]byte, 4*1024*1024))

	h.Remove(path)

	// Load empty
	_, err := db.Load(path)
	if err == nil {
		t.Fatalf("Should not have loaded a missing file")
	}
}

func BackendTestListFiles(t *testing.T, db StorageBackend, h BackendTestHarness) {
	folder := h.MakeFolder(t.Name())
	h.MakeFolder(filepath.Join(folder, "2017-11-28"))
	h.MakeFolder(filepath.Join(folder, "2018-11-28"))
	h.MakeFolder(filepath.Join(folder, "2019-11-28"))
	h.MakeFile(filepath.Join(folder, "metadata.data"), []byte{})
	h.MakeFile(filepath.Join(folder, "2019-11-28", "certs.data"), []byte{})

	var expectedFolders []string
	var expectedFiles []string

	err := db.List(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		removeFromList := func(s []string, i int) []string {
			s[i] = s[len(s)-1]
			return s[:len(s)-1]
		}

		if info.IsDir() {
			for i := 0; i < len(expectedFolders); i++ {
				if path == expectedFolders[i] {
					expectedFolders = removeFromList(expectedFolders, i)
					return nil
				}
			}

			// If it's the folder we're in, that's okay
			if path == folder {
				return nil
			}
		}

		for i := 0; i < len(expectedFiles); i++ {
			if path == expectedFiles[i] {
				expectedFiles = removeFromList(expectedFiles, i)
				return nil
			}
		}

		t.Errorf("Did't find %s", path)
		return nil
	})
	if err != nil {
		t.Error(err)
	}

	if len(expectedFolders) > 0 {
		t.Errorf("Didn't find folders: %+v", expectedFolders)
	}
	if len(expectedFiles) > 0 {
		t.Errorf("Didn't find files: %+v", expectedFiles)
	}
}

func BackendTestWriter(t *testing.T, db StorageBackend, h BackendTestHarness) {
	folder := h.MakeFolder(t.Name())
	path := filepath.Join(folder, "metadata.data")

	verifyText := func(text string) {
		data, err := db.Load(path)
		if err != nil {
			t.Error(err)
		}
		if string(data) != text {
			t.Errorf("Expected to read [%s] but found [%s]", text, data)
		}
	}

	truncWriter, err := db.Writer(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(truncWriter, "This is a string\n"); err != nil {
		t.Error(err)
	}
	truncWriter.Close()

	verifyText("This is a string\n")

	truncWriter, err = db.Writer(path, false)
	if err != nil {
		t.Error(err)
	}
	if _, err = io.WriteString(truncWriter, "This is another string\n"); err != nil {
		t.Error(err)
	}
	truncWriter.Close()

	verifyText("This is another string\n")

	appendWriter, err := db.Writer(path, true)
	if err != nil {
		t.Error(err)
	}
	if _, err = io.WriteString(appendWriter, "appended to the first\n"); err != nil {
		t.Error(err)
	}
	appendWriter.Close()

	verifyText("This is another string\nappended to the first\n")
}

func BackendTestReadWriter(t *testing.T, db StorageBackend, h BackendTestHarness) {
	folder := h.MakeFolder(t.Name())
	path := filepath.Join(folder, "metadata.data")

	verifyText := func(in io.Reader, text string) {
		data, err := ioutil.ReadAll(in)
		if err != nil {
			t.Error(err)
		}
		if string(data) != text {
			t.Errorf("Expected to read [%s] but found [%s]", text, data)
		}
	}

	appendWriter, err := db.ReadWriter(path)
	if err != nil {
		t.Fatal(err)
	}

	verifyText(appendWriter, "")
	if _, err = io.WriteString(appendWriter, "One"); err != nil {
		t.Error(err)
	}
	appendWriter.Close()

	appendWriter, err = db.ReadWriter(path)
	if err != nil {
		t.Error(err)
	}
	verifyText(appendWriter, "One")
	if _, err = io.WriteString(appendWriter, ", Two"); err != nil {
		t.Error(err)
	}
	appendWriter.Close()

	appendWriter, err = db.ReadWriter(path)
	if err != nil {
		t.Error(err)
	}
	verifyText(appendWriter, "One, Two")
	if _, err = io.WriteString(appendWriter, ", Three"); err != nil {
		t.Error(err)
	}
	appendWriter.Close()

	appendWriter, err = db.ReadWriter(path)
	if err != nil {
		t.Error(err)
	}
	verifyText(appendWriter, "One, Two, Three")
	appendWriter.Close()
}

func BackendTestAutoCreateFolders(t *testing.T, db StorageBackend, h BackendTestHarness) {
	folder := h.MakeFolder(t.Name())
	readwriterpath := filepath.Join(folder, "ReadWriter/file")
	readwriter, err := db.ReadWriter(readwriterpath)
	if err != nil {
		t.Error(err)
	}
	defer readwriter.Close()

	writerpath := filepath.Join(folder, "Writer/file")
	writer, err := db.Writer(writerpath, false)
	if err != nil {
		t.Error(err)
	}
	defer writer.Close()

	storepath := filepath.Join(folder, "Store/file")
	if err := db.Store(storepath, []byte{}); err != nil {
		t.Error(err)
	}
}