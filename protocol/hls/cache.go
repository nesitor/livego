package hls

import (
	"bytes"
	"container/list"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"strings"
	"sync"
)

const (
	maxTSCacheNum = 3
)

var (
	ErrNoKey = fmt.Errorf("No key for cache")
)

type TSCacheItem struct {
	id   string
	num  int
	lock sync.RWMutex
	ll   *list.List
	cll  *list.List
	lm   map[string]TSItem
	clm  map[string]TSItem
}

func NewTSCacheItem(id string) *TSCacheItem {
	return &TSCacheItem{
		id:  id,
		ll:  list.New(),
		cll: list.New(),
		num: maxTSCacheNum,
		lm:  make(map[string]TSItem),
		clm: make(map[string]TSItem),
	}
}

func (tcCacheItem *TSCacheItem) ID() string {
	return tcCacheItem.id
}

// TODO: found data race, fix it
func (tcCacheItem *TSCacheItem) GenM3U8PlayList() ([]byte, error) {
	var seq int
	var getSeq bool
	var maxDuration int
	m3u8body := bytes.NewBuffer(nil)
	for e := tcCacheItem.ll.Front(); e != nil; e = e.Next() {
		key := e.Value.(string)
		v, ok := tcCacheItem.lm[key]
		if ok {
			if v.Duration > maxDuration {
				maxDuration = v.Duration
			}
			if !getSeq {
				getSeq = true
				seq = v.SeqNum
			}
			fmt.Fprintf(m3u8body, "#EXTINF:%.3f,\n%s\n", float64(v.Duration)/float64(1000), v.Name)
		}
	}
	w := bytes.NewBuffer(nil)
	fmt.Fprintf(w,
		"#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-ALLOW-CACHE:NO\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:%d\n\n",
		maxDuration/1000+1, seq)
	w.Write(m3u8body.Bytes())
	return w.Bytes(), nil
}

func (tcCacheItem *TSCacheItem) GenCompleteM3U8PlayList(file string) ([]byte, error) {
	var seq int
	var getSeq bool
	var maxDuration int
	m3u8body := bytes.NewBuffer(nil)
	for e := tcCacheItem.cll.Front(); e != nil; e = e.Next() {
		key := e.Value.(string)
		v, ok := tcCacheItem.clm[key]
		if ok {
			if v.Duration > maxDuration {
				maxDuration = v.Duration
			}
			if !getSeq {
				getSeq = true
				seq = v.SeqNum
			}
			chunkName := tcCacheItem.getChunkName(v.Name)
			name := file + "/" + chunkName
			fmt.Fprintf(m3u8body, "#EXTINF:%.3f,\n%s\n", float64(v.Duration)/float64(1000), name)
		}
	}
	w := bytes.NewBuffer(nil)
	fmt.Fprintf(w,
		"#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-ALLOW-CACHE:NO\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:%d\n\n",
		maxDuration/1000+1, seq)
	w.Write(m3u8body.Bytes())
	fmt.Fprintf(w, "#EXT-X-ENDLIST")
	return w.Bytes(), nil
}

func (tcCacheItem *TSCacheItem) SaveCompleteM3U8PlayList(file string) error {
	filename := "tmp/" + file + ".m3u8"
	manifest, err := tcCacheItem.GenCompleteM3U8PlayList(file)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filename, manifest, 0644)
	return err
}

func (tcCacheItem *TSCacheItem) SetItem(key string, item TSItem) {
	if tcCacheItem.ll.Len() == tcCacheItem.num {
		e := tcCacheItem.ll.Front()
		tcCacheItem.ll.Remove(e)
		k := e.Value.(string)
		delete(tcCacheItem.lm, k)
	}
	tcCacheItem.lm[key] = item
	tcCacheItem.clm[key] = item
	tcCacheItem.ll.PushBack(key)
	tcCacheItem.cll.PushBack(key)
}

func (tcCacheItem *TSCacheItem) GetItem(key string) (TSItem, error) {
	item, ok := tcCacheItem.lm[key]
	if !ok {
		return item, ErrNoKey
	}
	return item, nil
}

func (tcCacheItem *TSCacheItem) SaveItem(appName string, key string, item TSItem) error {
	path := "tmp/" + appName + "/"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			log.Error("Creating AppName directory err: ", err)
		}
	}
	filename := path + key + ".ts"
	log.Info("Saving chunk: ", filename)
	err := ioutil.WriteFile(filename, item.Data, 0644)
	return err
}

func (tcCacheItem *TSCacheItem) getChunkName(s string) string {
	ss := strings.Split(s, "/")
	return ss[len(ss)-1]
}
