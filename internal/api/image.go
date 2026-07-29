package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/kdomanski/iso9660/util"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ListImages Get a list of all images
// @Summary Get all images
// @Tags images
// @Accept  json
// @Produce  json
// @Success 200 {array} model.Image
// @Failure 500 {object} model.APIError
// @Router /images [get]
func ListImages(c *gin.Context) { List[model.Image](c) }

// GetImage Get an existing image
// @Summary Get an existing image
// @Tags images
// @Accept  json
// @Produce  json
// @Param  id path int true "Image ID"
// @Success 200 {object} model.Image
// @Failure 400 {object} model.APIError
// @Failure 404 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /images/{id} [get]
func GetImage(c *gin.Context) { Get[model.Image](c) }

// CreateImage Create a new images
// @Summary Create a new image
// @Tags images
// @Accept  json
// @Produce  json
// @Param item body model.ImageForm true "Add image"
// @Success 200 {object} model.Image
// @Failure 400 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /images [post]
func CreateImage(c *gin.Context) {

	f, err := c.MultipartForm()
	if err != nil {
		Error(c, http.StatusInternalServerError, err) // 500
		return
	}

	files := f.File["file[]"]

	for _, file := range files {

		filename := file.Filename

		item := model.Image{}
		item.ISOImage = filepath.Base(file.Filename)
		item.Path = path.Join(".", "images", filename)
		item.Hash = c.PostForm("hash")
		item.Description = c.PostForm("description")

		if err := os.MkdirAll(filepath.Dir(item.Path), 0o755); err != nil {
			Error(c, http.StatusInternalServerError, err) // 500
			return
		}

		_, err = SaveUploadedFile(file, item.Path)
		if err != nil {
			Error(c, http.StatusInternalServerError, err) // 500
			return
		}

		if item.Hash == "" {
			logrus.WithFields(logrus.Fields{
				"Hash": item.Hash,
			}).Warning("Image uploaded with no hash, please consider using a hash to avoid image corruption")
		} else {
			logrus.WithFields(logrus.Fields{
				"Hash": item.Hash,
			}).Warning("Image uploaded with hash, comparing hash!")

			f, err := os.Open(item.Path)
			if err != nil {
				Error(c, http.StatusInternalServerError, err) // 500
				return
			}

			h := sha256.New()
			_, copyErr := io.Copy(h, f)
			closeErr := f.Close()
			if copyErr != nil {
				Error(c, http.StatusInternalServerError, copyErr) // 500
				return
			}
			if closeErr != nil {
				Error(c, http.StatusInternalServerError, closeErr) // 500
				return
			}

			if hex.EncodeToString(h.Sum(nil)) != item.Hash {
				err := fmt.Errorf("hash was invalid")
				Error(c, http.StatusBadRequest, err) // 400
				_ = os.Remove(item.Path)
				return
			}

		}

		f, err := os.Open(item.Path)
		if err != nil {
			Error(c, http.StatusInternalServerError, err) // 500
			return
		}
		defer func() { _ = f.Close() }()

		//strip the filextension, eg. vmware.iso = vmware
		fn := strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename))
		//merge into filepath
		fp := path.Join(".", "images", fn)

		if err = util.ExtractImageToDirectory(f, fp); err != nil {
			Error(c, http.StatusInternalServerError, fmt.Errorf("failed to extract image: %w", err)) // 500
			return
		}

		//remove the file
		err = os.Remove(item.Path)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Debug("image")
		}

		//update item.Path
		item.Path = fp

		// get size of extracted dir

		size, err := dirSize(fp)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Debug("image")
		}

		item.Size = size

		/*
			mime, err := mimetype.DetectFile(item.StoragePath)
			if err != nil {
				Error(c, http.StatusInternalServerError, err) // 500
				return
			}
			item.Type = mime.String()
			item.Extension = mime.Extension()
		*/

		if result := store.DB.Table("images").Create(&item); result.Error != nil {
			Error(c, http.StatusInternalServerError, result.Error) // 500
			return
		}
		logrus.WithFields(logrus.Fields{
			"id":          item.ID,
			"image":       item.ISOImage,
			"path":        item.Path,
			"size":        item.Size,
			"description": item.Description,
		}).Info("image")
		c.JSON(http.StatusOK, item) // 200
	}
}

func SaveUploadedFile(file *multipart.FileHeader, dst string) (int64, error) {
	src, err := file.Open()
	if err != nil {
		return -1, err
	}
	defer func() { _ = src.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return -1, err
	}
	defer func() { _ = out.Close() }()

	n, err := io.Copy(out, src)
	return n, err
}

// UpdateImage Update an existing image
// @Summary Update an existing image
// @Tags images
// @Accept  json
// @Produce  json
// @Param  id path int true "Image ID"
// @Param  item body model.ImageForm true "Update an image"
// @Success 200 {object} model.Image
// @Failure 400 {object} model.APIError
// @Failure 404 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /images/{id} [patch]
func UpdateImage(c *gin.Context) {
	Update(c, func(f model.ImageForm) model.Image { return model.Image{ImageForm: f} })
}

// DeleteImage Remove an existing image
// @Summary Remove an existing image
// @Tags images
// @Accept  json
// @Produce  json
// @Param  id path int true "Image ID"
// @Success 204
// @Failure 404 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /images/{id} [delete]
func DeleteImage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	// Load the item
	var item model.Image
	if res := store.DB.First(&item, id); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			Error(c, http.StatusNotFound, fmt.Errorf("not found")) // 404
		} else {
			Error(c, http.StatusInternalServerError, res.Error) // 500
		}
		return
	}

	//check if any group is using the image
	var group model.Group
	store.DB.First(&group, "image_id = ?", item.ID)

	if group.Name != "" {
		c.JSON(http.StatusConflict, "the image is being used by groups, please re-assign the groups to another image and then delete the image")
	} else {
		// Delete it
		//remove the entire directory and all files in it
		// This used to log.Fatal, killing the daemon because one image
		// directory could not be removed.
		if err := os.RemoveAll(item.Path); err != nil {
			Error(c, http.StatusInternalServerError, err) // 500
			return
		}

		// remove record from database
		if res := store.DB.Delete(&item); res.Error != nil {
			Error(c, http.StatusInternalServerError, res.Error) // 500
			return
		}

		c.JSON(http.StatusNoContent, gin.H{}) //204
	}

}

func WriteToFile(filename string, data string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	_, err = io.WriteString(file, data)
	if err != nil {
		return err
	}
	return file.Sync()
}

func GetInterfaceIpv4Addr(interfaceName string) (addr string, err error) {
	var (
		ief      *net.Interface
		addrs    []net.Addr
		ipv4Addr net.IP
	)
	if ief, err = net.InterfaceByName(interfaceName); err != nil { // get interface
		return
	}
	if addrs, err = ief.Addrs(); err != nil { // get addresses
		return
	}
	for _, addr := range addrs { // get ipv4 address
		if ipv4Addr = addr.(*net.IPNet).IP.To4(); ipv4Addr != nil {
			break
		}
	}
	if ipv4Addr == nil {
		return "", fmt.Errorf("interface %s does not have an ipv4 address", interfaceName)
	}
	return ipv4Addr.String(), nil
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	// convert byte to mb
	size = size / 1024 / 1024
	return size, err
}
