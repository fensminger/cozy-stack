// Package files is the HTTP frontend of the vfs package. It exposes
// an HTTP api to manipulate the filesystem and offer all the
// possibilities given by the vfs.
package files

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cozy/cozy-stack/vfs"
	"github.com/cozy/cozy-stack/web/jsonapi"
	"github.com/cozy/cozy-stack/web/middlewares"
	"github.com/gin-gonic/gin"
)

// DefaultContentType is used for files uploaded with no content-type
const DefaultContentType = "application/octet-stream"

// CreationHandler handle all POST requests on /files/:folder-id
// aiming at creating a new document in the FS. Given the Type
// parameter of the request, it will either upload a new file or
// create a new directory.
//
// swagger:route POST /files/:folder-id files uploadFileOrCreateDir
func CreationHandler(c *gin.Context) {
	vfsC, err := getVfsContext(c)
	if err != nil {
		return
	}

	docType, err := vfs.ParseDocType(c.Query("Type"))
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	var doc jsonapi.JSONApier
	switch docType {
	case vfs.FileDocType:
		doc, err = createFileHandler(c, vfsC)
	case vfs.FolderDocType:
		doc, err = createDirectoryHandler(c, vfsC)
	default:
		err = vfs.ErrDocTypeInvalid
	}

	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	data, err := doc.ToJSONApi()
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	c.Data(http.StatusCreated, jsonapi.ContentType, data)
}

func createFileHandler(c *gin.Context, vfsC *vfs.Context) (doc *vfs.FileDoc, err error) {
	doc, err = fileDocFromReq(
		c,
		c.Query("Name"),
		c.Param("folder-id"),
		parseTags(c.Query("Tags")),
	)
	if err != nil {
		return
	}

	err = vfs.CreateFileAndUpload(vfsC, doc, c.Request.Body)
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	return
}

func createDirectoryHandler(c *gin.Context, vfsC *vfs.Context) (doc *vfs.DirDoc, err error) {
	doc, err = vfs.NewDirDoc(
		c.Query("Name"),
		c.Param("folder-id"),
		parseTags(c.Query("Tags")),
	)
	if err != nil {
		return
	}

	err = vfs.CreateDirectory(vfsC, doc)
	if err != nil {
		return
	}

	return
}

// OverwriteFileContentHandler handles PUT requests on /files/:file-id
// to overwrite the content of a file given its identifier.
//
// swagger:route PUT /files/:file-id files overwriteFileContent
func OverwriteFileContentHandler(c *gin.Context) {
	var err error

	vfsC, err := getVfsContext(c)
	if err != nil {
		return
	}

	var oldDoc *vfs.FileDoc
	var newDoc *vfs.FileDoc

	oldDoc, err = vfs.GetFileDoc(vfsC, c.Param("file-id"))
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	newDoc, err = fileDocFromReq(
		c,
		oldDoc.Name,
		oldDoc.FolderID,
		oldDoc.Tags,
	)
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	ifMatch := c.Request.Header.Get("If-Match")
	if ifMatch != "" && oldDoc.Rev() != ifMatch {
		jsonapi.AbortWithError(c, jsonapi.PreconditionFailed("If-Match", fmt.Errorf("Revision does not match.")))
		return
	}

	err = vfs.ModifyFileContent(vfsC, oldDoc, newDoc, c.Request.Body)
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	data, err := newDoc.ToJSONApi()
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	c.Data(http.StatusOK, jsonapi.ContentType, data)
}

// @TODO: get rid of this with jsonapi package
type jsonData struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Attrs *vfs.DocMetaAttributes `json:"attributes"`
}

type jsonDataContainer struct {
	Data *jsonData `json:"data"`
}

// ModificationHandler handles PATCH requests on /files/:file-id. It
// can be used to modify the file or directory metadata, as well as
// moving and renaming it in the filesystem.
func ModificationHandler(c *gin.Context) {
	var err error

	vfsC, err := getVfsContext(c)
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.InternalServerError(err))
		return
	}

	var container jsonDataContainer
	err = c.BindJSON(&container)
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.BadRequest(err))
		return
	}

	patchData := container.Data
	docType, err := vfs.ParseDocType(patchData.Type)
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	var doc jsonapi.JSONApier
	switch docType {
	case vfs.FileDocType:
		doc, err = modFileHandler(vfsC, c.Request, patchData)
	case vfs.FolderDocType:
		// @TODO
		err = fmt.Errorf("Not implemented")
	}

	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	data, err := doc.ToJSONApi()
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	c.Data(http.StatusOK, jsonapi.ContentType, data)
}

func modFileHandler(vfsC *vfs.Context, req *http.Request, patchData *jsonData) (jsonapi.JSONApier, error) {
	doc, err := vfs.GetFileDoc(vfsC, patchData.ID)
	if err != nil {
		return nil, err
	}

	ifMatch := req.Header.Get("If-Match")
	if ifMatch != "" && doc.Rev() != ifMatch {
		return nil, jsonapi.PreconditionFailed("If-Match", fmt.Errorf("Revision does not match."))
	}

	doc, err = vfs.ModifyFileMetadata(vfsC, doc, patchData.Attrs)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// ReadFileHandler handles all GET requests on /files/:file-id,
// /files/download and /files/metadata and dispatches to the right
// handler. See ReadMetadataHandler and ReadFileContentHandler.
func ReadFileHandler(c *gin.Context) {
	fileID := c.Param("file-id")

	// Path /files/metadata is handled specifically to read file
	// metadata informations
	if fileID == "metadata" {
		ReadMetadataHandler(c)
	} else {
		ReadFileContentHandler(c)
	}
}

// ReadMetadataHandler handles all GET requests on /files/metadata
// aiming at getting file metadata from its path.
//
// swagger:route GET /files/metadata files getFileMetadata
func ReadMetadataHandler(c *gin.Context) {
	var err error

	vfsC, err := getVfsContext(c)
	if err != nil {
		return
	}

	fileDoc, err := vfs.GetFileDocFromPath(vfsC, c.Query("Path"))
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	data, err := fileDoc.ToJSONApi()
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	c.Data(http.StatusOK, jsonapi.ContentType, data)
}

// ReadFileContentHandler handles all GET requests on /files/:file-id
// aiming at downloading a file. It serves two main purposes in this
// regard:
//  - downloading a file given its ID in inline mode
//  - downloading a file given its path in attachment mode on the
//    /files/download endpoint
//
// swagger:route GET /files/download files downloadFileByPath
// swagger:route GET /files/:file-id files downloadFileByID
func ReadFileContentHandler(c *gin.Context) {
	var err error

	vfsC, err := getVfsContext(c)
	if err != nil {
		return
	}

	fileID := c.Param("file-id")

	// Path /files/download is handled specifically to download file
	// form their path
	var doc *vfs.FileDoc
	var disposition string
	if fileID == "download" {
		disposition = "attachment"
		doc, err = vfs.GetFileDocFromPath(vfsC, c.Query("Path"))
	} else {
		disposition = "inline"
		doc, err = vfs.GetFileDoc(vfsC, fileID)
	}

	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}

	err = vfs.ServeFileContent(vfsC, doc, disposition, c.Request, c.Writer)

	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.WrapVfsError(err))
		return
	}
}

// Routes sets the routing for the files service
func Routes(router *gin.RouterGroup) {
	router.HEAD("/:file-id", ReadFileHandler)
	router.GET("/:file-id", ReadFileHandler)

	router.POST("/", CreationHandler)
	router.POST("/:folder-id", CreationHandler)

	router.PATCH("/:file-id", ModificationHandler)
	router.PUT("/:file-id", OverwriteFileContentHandler)
}

func getVfsContext(c *gin.Context) (*vfs.Context, error) {
	instance := middlewares.GetInstance(c)
	dbprefix := instance.GetDatabasePrefix()
	fs, err := instance.GetStorageProvider()
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.InternalServerError(err))
		return nil, err
	}
	vfsC := vfs.NewContext(fs, dbprefix)
	return vfsC, nil
}

func fileDocFromReq(c *gin.Context, name, folderID string, tags []string) (doc *vfs.FileDoc, err error) {
	header := c.Request.Header

	size, err := parseContentLength(header.Get("Content-Length"))
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.InvalidParameter("Content-Length", err))
		return
	}

	var md5Sum []byte
	if md5Str := header.Get("Content-MD5"); md5Str != "" {
		md5Sum, err = parseMD5Hash(md5Str)
	}
	if err != nil {
		jsonapi.AbortWithError(c, jsonapi.InvalidParameter("Content-MD5", err))
		return
	}

	executable := c.Query("Executable") == "true"
	mime, class := extractMimeAndClass(c.ContentType())
	doc, err = vfs.NewFileDoc(
		name,
		folderID,
		size,
		md5Sum,
		mime,
		class,
		executable,
		tags,
	)

	return
}

func parseTags(str string) (tags []string) {
	for _, tag := range strings.Split(str, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return
}

func parseMD5Hash(md5B64 string) ([]byte, error) {
	// Encoded md5 hash in base64 should at least have 22 caracters in
	// base64: 16*3/4 = 21+1/3
	//
	// The padding may add up to 2 characters (non useful). If we are
	// out of these boundaries we know we don't have a good hash and we
	// can bail immediatly.
	if len(md5B64) < 22 || len(md5B64) > 24 {
		return nil, fmt.Errorf("Given Content-MD5 is invalid")
	}

	md5Sum, err := base64.StdEncoding.DecodeString(md5B64)
	if err != nil || len(md5Sum) != 16 {
		return nil, fmt.Errorf("Given Content-MD5 is invalid")
	}

	return md5Sum, nil
}

func parseContentLength(contentLength string) (size int64, err error) {
	if contentLength == "" {
		size = -1
		return
	}

	size, err = strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		err = fmt.Errorf("Invalid content length")
	}
	return
}

func extractMimeAndClass(contentType string) (mime, class string) {
	if contentType == "" {
		contentType = DefaultContentType
	}

	charsetIndex := strings.Index(contentType, ";")
	if charsetIndex >= 0 {
		mime = contentType[:charsetIndex]
	} else {
		mime = contentType
	}

	// @TODO improve for specific mime types
	slashIndex := strings.Index(contentType, "/")
	if slashIndex >= 0 {
		class = contentType[:slashIndex]
	} else {
		class = contentType
	}

	return
}
