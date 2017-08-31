package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"regexp"

	"github.com/go-yaml/yaml"
	"github.com/gorilla/mux"
	"github.com/shogo82148/androidbinary/apk"
)

type ApkInfo struct {
	PackageName  string `json:"packageName"`
	MainActivity string `json:"mainActivity"`
	Version      struct {
		Code int    `json:"code"`
		Name string `json:"name"`
	} `json:"version"`
}

type IndexFileItem struct {
	Path string
	Info os.FileInfo
}

type HTTPStaticServer struct {
	Root            string
	Upload          bool
	Delete          bool
	MKDir           bool
	Title           string
	Theme           string
	PlistProxy      string
	GoogleTrackerId string
	AuthType        string

	indexes []IndexFileItem
	m       *mux.Router
}

func NewHTTPStaticServer(root string) *HTTPStaticServer {
	if root == "" {
		root = "./"
	}
	root = filepath.ToSlash(root)
	if !strings.HasSuffix(root, "/") {
		root = root + "/"
	}
	log.Printf("root path: %s\n", root)
	m := mux.NewRouter()
	s := &HTTPStaticServer{
		Root:  root,
		Theme: "black",
		m:     m,
	}

	go func() {
		time.Sleep(1 * time.Second)
		for {
			startTime := time.Now()
			log.Println("Started making search index")
			s.makeIndex()
			log.Printf("Completed search index in %v", time.Since(startTime))
			//time.Sleep(time.Second * 1)
			time.Sleep(time.Minute * 10)
		}
	}()

	m.HandleFunc("/-/status", s.hStatus)
	m.HandleFunc("/-/zip/{path:.*}", s.hZip)
	m.HandleFunc("/-/unzip/{zip_path:.*}/-/{path:.*}", s.hUnzip)
	m.HandleFunc("/-/json/{path:.*}", s.hJSONList)
	// routers for directory
	m.HandleFunc("/-/mkdir/{path:.*}", s.hMkdir).Methods("POST")
	// routers for checkout directory

	// routers for Apple *.ipa
	m.HandleFunc("/-/ipa/plist/{path:.*}", s.hPlist)
	m.HandleFunc("/-/ipa/link/{path:.*}", s.hIpaLink)

	// TODO: /ipa/info
	m.HandleFunc("/-/info/{path:.*}", s.hInfo)
	// routers for listing (directory or files) / uploading / deleting files
	m.HandleFunc("/{path:.*}", s.hIndex).Methods("GET", "HEAD")
	m.HandleFunc("/{path:.*}", s.hUpload).Methods("POST")
	m.HandleFunc("/{path:.*}", s.hEdit).Methods("PUT")
	m.HandleFunc("/{path:.*}", s.hDelete).Methods("DELETE")

	return s
}

func (s *HTTPStaticServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.m.ServeHTTP(w, r)
}

func (s *HTTPStaticServer) hIndex(w http.ResponseWriter, r *http.Request) {
	path := mux.Vars(r)["path"]
	relPath := filepath.Join(s.Root, path)

	if r.FormValue("raw") == "false" || isDir(relPath) {
		if r.Method == "HEAD" {
			return
		}
		tmpl.ExecuteTemplate(w, "index", s)
	} else {
		if r.FormValue("download") == "true" {
			w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(filepath.Base(path)))
		}
		http.ServeFile(w, r, relPath)
	}
}

func (s *HTTPStaticServer) hStatus(w http.ResponseWriter, r *http.Request) {
	data, _ := json.MarshalIndent(s, "", "    ")
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// create function to HTTPStaticServer for making directory
func (s *HTTPStaticServer) hMkdir(w http.ResponseWriter, req *http.Request) {
	path := mux.Vars(req)["path"]
	auth := s.readAccessConf(path)
	log.Printf("%#v", auth)
	if !auth.canMKDir(req) {
		http.Error(w, "Mkdir forbidden", http.StatusForbidden)
		return
	}
	// get folder name from request Body
	folderName := req.FormValue("folderName")
	folder := filepath.Join(s.Root, path, folderName)
	err := os.Mkdir(folder, 0731) // wxr-xr-x
	if err != nil {
		// if folder already exists
		if os.IsExist(err) {
			http.Error(w, "Mkdir forbidden: directory already exists", http.StatusForbidden)
		} else {
			http.Error(w, err.Error(), 500)
		}
		return
	}
	// create default auth file
	cfgFile := filepath.Join(folder, ".ghs.yml")
	file, createErr := os.Create(cfgFile)
	if createErr != nil {
		log.Printf("%#v", createErr)
	}
	file.WriteString("upload: true\ndelete: true\nmkdir: false")
	defer file.Close()

	w.Write([]byte("Success"))
}

// create function to HTTPStaticServer for editing file
func (s *HTTPStaticServer) hEdit(w http.ResponseWriter, req *http.Request) {
	// only can delete file now
	path := mux.Vars(req)["path"]
	auth := s.readAccessConf(path)
	log.Printf("%#v", auth)
	if !auth.canUpload(req) {
		// user can edit file only if has upload authority
		http.Error(w, "Edit forbidden", http.StatusForbidden)
		return
	}
	// get file content from request Body
	fileContent := req.FormValue("content")
	localPath := filepath.Join(s.Root, path)
	// if path is directory, can't edit
	if isDir(localPath) {
		http.Error(w, "Edit forbidden: directory can't be modified: "+localPath, http.StatusForbidden)
		return
	}
	// open file and over write
	err := ioutil.WriteFile(localPath, []byte(fileContent), 0666)
	if err != nil {
		// if open file fails
		if !os.IsExist(err) {
			http.Error(w, "Edit forbidden: file not exists", http.StatusForbidden)
		} else {
			http.Error(w, err.Error(), 500)
		}
		return
	}
	w.Write([]byte("Success"))
}

func (s *HTTPStaticServer) hDelete(w http.ResponseWriter, req *http.Request) {
	path := mux.Vars(req)["path"]
	auth := s.readAccessConf(path)
	log.Printf("%#v", auth)
	if !auth.canDelete(req) {
		http.Error(w, "Delete forbidden", http.StatusForbidden)
		return
	}
	localPath := filepath.Join(s.Root, path)
	// delete single file
	if isFile(localPath) {
		err := os.Remove(localPath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	} else if isDir(localPath) {
		// delete directory
		err := os.RemoveAll(localPath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}

	w.Write([]byte("Success"))
}

func (s *HTTPStaticServer) hUpload(w http.ResponseWriter, req *http.Request) {
	path := mux.Vars(req)["path"]
	dirpath := filepath.Join(s.Root, path)

	// check auth
	auth := s.readAccessConf(path)
	if !auth.canUpload(req) {
		http.Error(w, "Upload forbidden", http.StatusForbidden)
		return
	}

	file, header, err := req.FormFile("file")
	if err != nil {
		log.Println("Parse form file:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() {
		file.Close()
		req.MultipartForm.RemoveAll() // Seen from go source code, req.MultipartForm not nil after call FormFile(..)
	}()
	dstPath := filepath.Join(dirpath, header.Filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		log.Println("Create file:", err)
		http.Error(w, "File create "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		log.Println("Handle upload file:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"destination": dstPath,
	})
}

type FileJSONInfo struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Size    int64       `json:"size"`
	Path    string      `json:"path"`
	ModTime int64       `json:"mtime"`
	Extra   interface{} `json:"extra,omitempty"`
}

// path should be absolute
func parseApkInfo(path string) (ai *ApkInfo) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("parse-apk-info panic:", err)
		}
	}()
	apkf, err := apk.OpenFile(path)
	if err != nil {
		return
	}
	ai = &ApkInfo{}
	ai.MainActivity, _ = apkf.MainAcitivty()
	ai.PackageName = apkf.PackageName()
	ai.Version.Code = apkf.Manifest().VersionCode
	ai.Version.Name = apkf.Manifest().VersionName
	return
}

func (s *HTTPStaticServer) hInfo(w http.ResponseWriter, r *http.Request) {
	path := mux.Vars(r)["path"]
	relPath := filepath.Join(s.Root, path)
	if !isFile(relPath) {
		http.Error(w, "Not a file", 403)
		return
	}
	fi, err := os.Stat(relPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	fji := &FileJSONInfo{
		Name:    fi.Name(),
		Size:    fi.Size(),
		Path:    path,
		ModTime: fi.ModTime().UnixNano() / 1e6,
	}
	ext := filepath.Ext(path)
	switch ext {
	case ".md":
		fji.Type = "markdown"
	case ".apk":
		fji.Type = "apk"
		fji.Extra = parseApkInfo(relPath)
	default:
		fji.Type = "text"
	}
	data, _ := json.Marshal(fji)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *HTTPStaticServer) hZip(w http.ResponseWriter, r *http.Request) {
	path := mux.Vars(r)["path"]
	CompressToZip(w, filepath.Join(s.Root, path))
}

func (s *HTTPStaticServer) hUnzip(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zipPath, path := vars["zip_path"], vars["path"]
	ctype := mime.TypeByExtension(filepath.Ext(path))
	if ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
	err := ExtractFromZip(filepath.Join(s.Root, zipPath), path, w)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func genURLStr(r *http.Request, path string) *url.URL {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return &url.URL{
		Scheme: scheme,
		Host:   r.Host,
		Path:   path,
	}
}

func (s *HTTPStaticServer) hPlist(w http.ResponseWriter, r *http.Request) {
	path := mux.Vars(r)["path"]
	// rename *.plist to *.ipa
	if filepath.Ext(path) == ".plist" {
		path = path[0:len(path)-6] + ".ipa"
	}

	relPath := filepath.Join(s.Root, path)
	plinfo, err := parseIPA(relPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := &url.URL{
		Scheme: scheme,
		Host:   r.Host,
	}
	data, err := generateDownloadPlist(baseURL, path, plinfo)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/xml")
	w.Write(data)
}

func (s *HTTPStaticServer) hIpaLink(w http.ResponseWriter, r *http.Request) {
	path := mux.Vars(r)["path"]
	plistUrl := genURLStr(r, "/-/ipa/plist/"+path).String()
	if r.TLS == nil {
		// send plist to plistproxy and get a https link
		httpPlistLink := "http://" + r.Host + "/-/ipa/plist/" + path
		url, err := s.genPlistLink(httpPlistLink)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		plistUrl = url
		//plistUrl = strings.TrimSuffix(s.PlistProxy, "/") + "/" + r.Host + "/-/ipa/plist/" + path
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl.ExecuteTemplate(w, "ipa-install", map[string]string{
		"Name":      filepath.Base(path),
		"PlistLink": plistUrl,
	})
	// w.Write([]byte(fmt.Sprintf(
	// 	`<a href='itms-services://?action=download-manifest&url=%s'>Click this link to install</a>`,
	// 	plistUrl)))
}

func (s *HTTPStaticServer) genPlistLink(httpPlistLink string) (plistUrl string, err error) {
	// Maybe need a proxy, a little slowly now.
	pp := s.PlistProxy
	if pp == "" {
		pp = defaultPlistProxy
	}
	resp, err := http.Get(httpPlistLink)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	data, _ := ioutil.ReadAll(resp.Body)
	retData, err := http.Post(pp, "text/xml", bytes.NewBuffer(data))
	if err != nil {
		return
	}
	defer retData.Body.Close()

	jsonData, _ := ioutil.ReadAll(retData.Body)
	var ret map[string]string
	if err = json.Unmarshal(jsonData, &ret); err != nil {
		return
	}
	plistUrl = pp + "/" + ret["key"]
	return
}

func (s *HTTPStaticServer) hFileOrDirectory(w http.ResponseWriter, r *http.Request) {
	path := mux.Vars(r)["path"]
	http.ServeFile(w, r, filepath.Join(s.Root, path))
}

type HTTPFileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Type    string `json:"type"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime"`
}

type AccessTable struct {
	Regex string `yaml:"regex"`
	Allow bool   `yaml:"allow"`
}

type UserControl struct {
	Email string
	// Access bool
	Upload bool // upload file
	Delete bool // delete file
	MKDir  bool // create dir
}

type AccessConf struct {
	Upload       bool          `yaml:"upload" json:"upload"`
	Delete       bool          `yaml:"delete" json:"delete"`
	MKDir        bool          `yaml:"mkdir" json:"mkdir"`
	Users        []UserControl `yaml:"users" json:"users"`
	AccessTables []AccessTable `yaml:"accessTables"`
}

var reCache = make(map[string]*regexp.Regexp)

func (c *AccessConf) canAccess(fileName string) bool {
	for _, table := range c.AccessTables {
		pattern, ok := reCache[table.Regex]
		if !ok {
			pattern, _ = regexp.Compile(table.Regex)
			reCache[table.Regex] = pattern
		}
		// skip wrong format regex
		if pattern == nil {
			continue
		}
		if pattern.MatchString(fileName) {
			return table.Allow
		}
	}
	return true
}

func (c *AccessConf) canDelete(r *http.Request) bool {
	session, err := store.Get(r, defaultSessionName)
	if err != nil {
		return c.Delete
	}
	val := session.Values["user"]
	if val == nil {
		return c.Delete
	}
	userInfo := val.(*UserInfo)
	for _, rule := range c.Users {
		if rule.Email == userInfo.Email {
			return rule.Delete
		}
	}
	return c.Delete
}

func (c *AccessConf) canUpload(r *http.Request) bool {
	session, err := store.Get(r, defaultSessionName)
	if err != nil {
		return c.Upload
	}
	val := session.Values["user"]
	if val == nil {
		return c.Upload
	}
	userInfo := val.(*UserInfo)
	for _, rule := range c.Users {
		if rule.Email == userInfo.Email {
			return rule.Upload
		}
	}
	return c.Upload
}

/* function can mkdir */
func (c *AccessConf) canMKDir(r *http.Request) bool {
	session, err := store.Get(r, defaultSessionName)
	if err != nil {
		return c.MKDir
	}
	val := session.Values["user"]
	if val == nil {
		return c.MKDir
	}
	userInfo := val.(*UserInfo)
	for _, rule := range c.Users {
		if rule.Email == userInfo.Email {
			return rule.MKDir
		}
	}
	return c.MKDir
}

func (s *HTTPStaticServer) hJSONList(w http.ResponseWriter, r *http.Request) {
	requestPath := mux.Vars(r)["path"]
	localPath := filepath.Join(s.Root, requestPath)
	search := r.FormValue("search")
	auth := s.readAccessConf(requestPath)
	auth.Upload = auth.canUpload(r)
	auth.Delete = auth.canDelete(r)
	auth.MKDir = auth.canMKDir(r)

	// path string -> info os.FileInfo
	fileInfoMap := make(map[string]os.FileInfo, 0)

	if search != "" {
		results := s.findIndex(search)
		if len(results) > 50 { // max 50
			results = results[:50]
		}
		for _, item := range results {
			if filepath.HasPrefix(item.Path, requestPath) {
				fileInfoMap[item.Path] = item.Info
			}
		}
	} else {
		infos, err := ioutil.ReadDir(localPath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		for _, info := range infos {
			fileInfoMap[filepath.Join(requestPath, info.Name())] = info
		}
	}

	// turn file list -> json
	lrs := make([]HTTPFileInfo, 0)
	for path, info := range fileInfoMap {
		if !auth.canAccess(info.Name()) {
			continue
		}
		lr := HTTPFileInfo{
			Name:    info.Name(),
			Path:    path,
			ModTime: info.ModTime().UnixNano() / 1e6,
		}
		if search != "" {
			name, err := filepath.Rel(requestPath, path)
			if err != nil {
				log.Println(requestPath, path, err)
			}
			lr.Name = filepath.ToSlash(name) // fix for windows
		}
		if info.IsDir() {
			name := deepPath(localPath, info.Name())
			lr.Name = name
			lr.Path = filepath.Join(filepath.Dir(path), name)
			lr.Type = "dir"
			lr.Size = s.historyDirSize(lr.Path)
		} else {
			lr.Type = "file"
			lr.Size = info.Size() // formatSize(info)
		}
		lrs = append(lrs, lr)
	}

	data, _ := json.Marshal(map[string]interface{}{
		"files": lrs,
		"auth":  auth,
	})
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

var dirSizeMap = make(map[string]int64)

func (s *HTTPStaticServer) makeIndex() error {
	var indexes = make([]IndexFileItem, 0)
	var err = filepath.Walk(s.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("WARN: Visit path: %s error: %v", strconv.Quote(path), err)
			return filepath.SkipDir
			// return err
		}
		if info.IsDir() {
			return nil
		}

		path, _ = filepath.Rel(s.Root, path)
		path = filepath.ToSlash(path)
		indexes = append(indexes, IndexFileItem{path, info})
		return nil
	})
	s.indexes = indexes
	dirSizeMap = make(map[string]int64)
	return err
}

func (s *HTTPStaticServer) historyDirSize(dir string) int64 {
	var size int64
	if size, ok := dirSizeMap[dir]; ok {
		return size
	}
	for _, fitem := range s.indexes {
		if filepath.HasPrefix(fitem.Path, dir) {
			size += fitem.Info.Size()
		}
	}
	dirSizeMap[dir] = size
	return size
}

func (s *HTTPStaticServer) findIndex(text string) []IndexFileItem {
	ret := make([]IndexFileItem, 0)
	for _, item := range s.indexes {
		ok := true
		// search algorithm, space for AND
		for _, keyword := range strings.Fields(text) {
			needContains := true
			if strings.HasPrefix(keyword, "-") {
				needContains = false
				keyword = keyword[1:]
			}
			if keyword == "" {
				continue
			}
			ok = (needContains == strings.Contains(strings.ToLower(item.Path), strings.ToLower(keyword)))
			if !ok {
				break
			}
		}
		if ok {
			ret = append(ret, item)
		}
	}
	return ret
}

func (s *HTTPStaticServer) defaultAccessConf() AccessConf {
	return AccessConf{
		Upload: s.Upload,
		Delete: s.Delete,
		MKDir:  s.MKDir,
	}
}

func (s *HTTPStaticServer) readAccessConf(requestPath string) (ac AccessConf) {
	requestPath = filepath.Clean(requestPath)
	if requestPath == "/" || requestPath == "" || requestPath == "." {
		ac = s.defaultAccessConf()
	} else {
		parentPath := filepath.Dir(requestPath)
		ac = s.readAccessConf(parentPath)
	}
	relPath := filepath.Join(s.Root, requestPath)
	if isFile(relPath) {
		relPath = filepath.Dir(relPath)
	}
	cfgFile := filepath.Join(relPath, ".ghs.yml")
	data, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("Err read .ghs.yml: %v", err)
	}
	err = yaml.Unmarshal(data, &ac)
	if err != nil {
		log.Printf("Err format .ghs.yml: %v", err)
	}
	return
}

func deepPath(basedir, name string) string {
	isDir := true
	// loop max 5, incase of for loop not finished
	maxDepth := 5
	for depth := 0; depth <= maxDepth && isDir; depth += 1 {
		finfos, err := ioutil.ReadDir(filepath.Join(basedir, name))
		if err != nil || len(finfos) != 1 {
			break
		}
		if finfos[0].IsDir() {
			name = filepath.ToSlash(filepath.Join(name, finfos[0].Name()))
		} else {
			break
		}
	}
	return name
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsDir()
}
