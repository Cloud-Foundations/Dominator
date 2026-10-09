package fstree

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/net/http"
)

type getter struct {
	baseUrl    string
	httpClient http.Client
	logger     log.DebugLogger
}

type walker struct {
	WalkParams
	errorMutex sync.Mutex
	error      error
	waitGroup  sync.WaitGroup
}

func decodeTree(r io.Reader) (*Tree, error) {
	scanner := bufio.NewScanner(r)
	var tree Tree
	for scanner.Scan() {
		var entry TreeEntry
		if err := decodeTreeEntry(scanner.Text(), &entry); err != nil {
			return nil, err
		}
		tree.Entries = append(tree.Entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning tree: %s", err)
	}
	return &tree, nil
}

func decodeTreeEntry(line string, entry *TreeEntry) error {
	// <filename:str>/<type:u32>/<perms:u32>/<uid:u32>/<gid:u32>/<size:u64>/<hash:u512>\n
	splitLine := strings.Split(line, "/")
	if len(splitLine) != 7 {
		return fmt.Errorf("number of segments: %d != 7: %s",
			len(splitLine), line)
	}
	entry.Filename = splitLine[0]
	if err := parseUint32(splitLine[1], &entry.Type); err != nil {
		return err
	}
	if err := parseUint32(splitLine[2], &entry.Permissions); err != nil {
		return err
	}
	if err := parseUint32(splitLine[3], &entry.UserId); err != nil {
		return err
	}
	if err := parseUint32(splitLine[4], &entry.GroupId); err != nil {
		return err
	}
	if err := parseUint64(splitLine[5], &entry.Size); err != nil {
		return err
	}
	if err := entry.Hash.UnmarshalText([]byte(splitLine[6])); err != nil {
		return err
	}
	return nil
}

func getTree(treeUrl string) (*Tree, error) {
	rc, _, err := http.GetReader(nil, treeUrl)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return decodeTree(rc)
}

func parseUint32(str string, value *uint32) error {
	if v, err := strconv.ParseUint(str, 10, 32); err != nil {
		return err
	} else {
		*value = uint32(v)
		return nil
	}
}

func parseUint64(str string, value *uint64) error {
	if v, err := strconv.ParseUint(str, 10, 32); err != nil {
		return err
	} else {
		*value = v
		return nil
	}
}

func splitTreeUrl(treeUrl string) (string, string, error) {
	u, err := url.Parse(treeUrl)
	if err != nil {
		return "", "", err
	}
	dir := path.Dir(u.Path)
	hashString := path.Base(u.Path)
	if path.Base(dir) != "tree" {
		return "", "", fmt.Errorf("not a tree")
	}
	u.Path = path.Dir(dir)
	return u.String(), hashString, nil
}

func newGetter(params GetterParams) (Getter, error) {
	params.Logger.Debugf(0, "Creating Getter with concurrency=%d\n",
		cap(params.IoSemaphore))
	return &getter{
		baseUrl:    params.BaseUrl,
		httpClient: http.NewLimitedConcurrencyClient(params.IoSemaphore),
		logger:     params.Logger,
	}, nil
}

func (g *getter) GetBlobData(hashVal hash.Hash) ([]byte, error) {
	rc, _, err := g.GetBlobReader(hashVal)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (g *getter) GetBlobReader(hashVal hash.Hash) (
	io.ReadCloser, uint64, error) {
	return g.httpClient.GetReader(fmt.Sprintf("%s/blob/%x", g.baseUrl, hashVal))
}

func (g *getter) GetTree(hashVal hash.Hash) (*Tree, error) {
	rc, _, err := g.GetTreeReader(hashVal)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return decodeTree(rc)
}

func (g *getter) GetTreeReader(hashVal hash.Hash) (
	io.ReadCloser, uint64, error) {
	return g.httpClient.GetReader(
		fmt.Sprintf("%s/tree/%x", g.baseUrl, hashVal))
}

func walkTree(params WalkParams) error {
	rootTree, err := getTree(params.TreeUrl)
	if err != nil {
		return err
	}
	w := &walker{WalkParams: params}
	w.waitGroup.Add(1)
	w.processTree("/", rootTree)
	w.waitGroup.Wait()
	return w.error
}

func (w *walker) setError(err error) {
	if err == nil {
		return
	}
	w.errorMutex.Lock()
	if w.error == nil {
		w.error = err
	}
	w.errorMutex.Unlock()
}

func (w *walker) processTree(dirname string, tree *Tree) {
	defer w.waitGroup.Done()
	if w.error != nil {
		return
	}
	sort.Slice(tree.Entries, func(left, right int) bool {
		return tree.Entries[left].Filename < tree.Entries[right].Filename
	})
	for _, entry := range tree.Entries {
		if err := w.Function(w.Getter, dirname, &entry); err != nil {
			w.setError(err)
			return
		}
		if entry.Size < 1 {
			continue
		}
		if entry.Type != TypeTree {
			continue
		}
		if entryTree, err := w.Getter.GetTree(entry.Hash); err != nil {
			w.setError(err)
			return
		} else {
			subdir := path.Join(dirname, entry.Filename)
			w.waitGroup.Add(1)
			go w.processTree(subdir, entryTree)
		}
	}
}
