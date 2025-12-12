package uefi

import (
	"net"
	"net/http"
	"os"
	"fmt"
	"regexp"
	"strings"
	"strconv"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/maxiepax/go-via/db"
	"github.com/maxiepax/go-via/config"
	"github.com/maxiepax/go-via/models"
	"gorm.io/gorm/clause"
	"github.com/sirupsen/logrus"
)

/*
func UEFImboot() func(c *gin.Context) {
	return func(c *gin.Context) {
		//ip := net.ParseIP(c.ClientIP())
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		//get the object that correlates with the ip
		var host models.Host
		db.DB.Preload(clause.Associations).First(&host, "ip = ?", ip)

		//get the image info that correlates with the host
		var image models.Image
		db.DB.First(&image, "id = ?", host.Group.ImageID)

		//check these paths if the file exists.
		imagePath := image.Path
		paths := []string{"/EFI/BOOT/BOOTX64.EFI", "/EFI/BOOT/BOOTAA64.EFI", "/MBOOT.EFI", "/mboot.efi", "/efi/boot/bootx64.efi", "/efi/boot/bootaa64.efi"}

		for _, v := range paths {
			if _, err := os.Stat(imagePath + v); err == nil {
				c.File(imagePath + v)
			}
		}
	}
}

func UEFIcrypto64() func(c *gin.Context) {
	return func(c *gin.Context) {
		//ip := net.ParseIP(c.ClientIP())
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		//get the object that correlates with the ip
		var host models.Host
		db.DB.Preload(clause.Associations).First(&host, "ip = ?", ip)

		//get the image info that correlates with the host
		var image models.Image
		db.DB.First(&image, "id = ?", host.Group.ImageID)

		//check these paths if the file exists.
		imagePath := image.Path
		//check these paths if the file exists.
		paths := []string{"/EFI/BOOT/CRYPTO64.EFI", "/efi/boot/crypto64.efi"}

		for _, v := range paths {
			if _, err := os.Stat(imagePath + v); err == nil {
				c.File(imagePath + v)
			}
		}
	}
}
*/

func Files(conf *config.Config) func(c *gin.Context) {
	return func(c *gin.Context) {
		filepath := c.Param("filepath")

		//ip := net.ParseIP(c.ClientIP())
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		//get the object that correlates with the ip
		var host models.Host
		db.DB.Preload(clause.Associations).First(&host, "ip = ?", ip)

		//get the image info that correlates with the host
		var image models.Image
		db.DB.First(&image, "id = ?", host.Group.ImageID)

		//c.String(http.StatusOK, filepath)

		switch filepath {
		case "/mboot.efi":
			logrus.WithFields(logrus.Fields{
				ip: "requesting mboot.efi",
			}).Info("uefi-https")
			logrus.WithFields(logrus.Fields{
				"id":           host.ID,
				"percentage":   10,
				"progresstext": "mboot.efi",
			}).Info("progress")
			filepath, _ = UEFImboot(image.Path)
			host.Progress = 10
			host.Progresstext = "mboot.efi"
			db.DB.Save(&host)
			c.File(filepath)
            return
		case "/crypto64.efi":
			logrus.WithFields(logrus.Fields{
				ip: "requesting crypto64.efi",
			}).Info("uefi-https")
			logrus.WithFields(logrus.Fields{
				"id":           host.ID,
				"percentage":   12,
				"progresstext": "crypto64.efi",
			}).Info("progress")
			filepath, _ = UEFIcrypto64(image.Path)
			host.Progress = 12
			host.Progresstext = "crypto64.efi"
			db.DB.Save(&host)
			c.File(filepath)
            return
		case "boot.cfg", "/boot.cfg":
			localAddr, _ := c.Request.Context().Value(http.LocalAddrContextKey).(net.Addr)
			localipport := localAddr.String()
			localip, _, _ := net.SplitHostPort(localipport)
			remoteip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
            bootconfig, err := serveBootCfg(filepath, host, image, conf, localip, remoteip)
            if err != nil {
                logrus.WithFields(logrus.Fields{
                    "error": err,
                }).Error("boot.cfg")
                c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate boot.cfg"})
                return
            }
            c.Data(http.StatusOK, "application/octet-stream", bootconfig)
            return
		default:
			//if no case matches, chroot to /images
			if _, err := os.Stat( image.Path + filepath); err == nil {
				filepath = image.Path + filepath
				logrus.WithFields(logrus.Fields{
					"lowercase file": filepath,
				}).Debug("uefi-https")
			} else {
				filepath = image.Path + strings.ToUpper(filepath)
				logrus.WithFields(logrus.Fields{
					"uppercase file": filepath,
				}).Debug("uefi-https")
			}
		}
		c.File(filepath)
        return
	}
}

func UEFImboot(imagePath string) (string, error) {
	//check these paths if the file exists.
	paths := []string{"/EFI/BOOT/BOOTX64.EFI", "/EFI/BOOT/BOOTAA64.EFI", "/MBOOT.EFI", "/mboot.efi", "/efi/boot/bootx64.efi", "/efi/boot/bootaa64.efi"}

	for _, v := range paths {
		if _, err := os.Stat(imagePath + v); err == nil {
			return imagePath + v, nil
		}
	}

	//couldn't find the file
	return "", fmt.Errorf("could not locate a mboot.efi")
}

func UEFIcrypto64(imagePath string) (string, error) {
	//check these paths if the file exists.
	paths := []string{"/EFI/BOOT/CRYPTO64.EFI", "/efi/boot/crypto64.efi"}

	for _, v := range paths {
		if _, err := os.Stat(imagePath + v); err == nil {
			return imagePath + v, nil
		}
	}

	//couldn't find the file
	return "", fmt.Errorf("could not locate a crypto64.efi")
}

func ipv4MaskString(m []byte) string {
	if len(m) != 4 {
		panic("ipv4Mask: len must be 4 bytes")
	}

	return fmt.Sprintf("%d.%d.%d.%d", m[0], m[1], m[2], m[3])
}

func serveBootCfg(filepath string, host models.Host, image models.Image, conf *config.Config, localip string, remoteip string) ([]byte, error) {
	//if the filepath is boot.cfg, or /boot.cfg, we serve the boot cfg that belongs to that build. unfortunately, it seems boot.cfg or /boot.cfg varies in builds.

	logrus.WithFields(logrus.Fields{
		remoteip: "requesting boot.cfg",
	}).Info("uefi-https")
	logrus.WithFields(logrus.Fields{
		"id":           host.ID,
		"percentage":   15,
		"progresstext": "installation",
	}).Info("progress")
	host.Progress = 15
	host.Progresstext = "installation"
	db.DB.Save(&host)

	bc, err := os.ReadFile(image.Path + "/BOOT.CFG")
	if err != nil {
		logrus.Warn(err)
		return nil, err
	}

	// strip slashes from paths in file
	re := regexp.MustCompile("/")
	bc = re.ReplaceAllLiteral(bc, []byte(""))

	// remove cdromBoot from kernelopt line
	re = regexp.MustCompile("cdromBoot")
	bc = re.ReplaceAllLiteral(bc, []byte(""))

	// add kickstart path to kernelopt
	re = regexp.MustCompile("kernelopt=.*")
	o := re.Find(bc)
	bc = re.ReplaceAllLiteral(bc, append(o, []byte(" ks=https://"+localip+":"+strconv.Itoa(conf.Port)+"/ks.cfg")...))

	// append the mac address of the hardware interface to ensure ks.cfg request comes from the right interface, along with ip, netmask and gateway.
	nm := net.CIDRMask(host.Pool.Netmask, 32)
	netmask := ipv4MaskString(nm)

	re = regexp.MustCompile("kernelopt=.*")
	o = re.Find(bc)
	bc = re.ReplaceAllLiteral(bc, append(o, []byte(" netdevice="+host.Mac+" ip="+host.IP+" netmask="+netmask+" gateway="+host.Pool.Gateway)...))

	// if vlan is configured for the group, append the vlan to kernelopts
	if host.Group.Vlan != "" {
		re = regexp.MustCompile("kernelopt=.*")
		o = re.Find(bc)
		bc = re.ReplaceAllLiteral(bc, append(o, []byte(" vlanid="+host.Group.Vlan)...))
	}

	// load options from the group
	options := models.GroupOptions{}
	json.Unmarshal(host.Group.Options, &options)

	// if autopart is configured for the group, append autopart to kernelopt - https://kb.vmware.com/s/article/77009
	/*
		if options.AutoPart {
			re = regexp.MustCompile("kernelopt=.*")
			o = re.Find(bc)
			bc = re.ReplaceAllLiteral(bc, append(o, []byte(" autoPartitionOnlyOnceAndSkipSsd=true")...))
		}*/

	// add allowLegacyCPU=true to kernelopt
	re = regexp.MustCompile("kernelopt=.*")
	o = re.Find(bc)
	bc = re.ReplaceAllLiteral(bc, append(o, []byte(" allowLegacyCPU=true")...))


	// replace prefix with prefix=foldername
	re = regexp.MustCompile("prefix=")
	o = re.Find(bc)
	bc = re.ReplaceAllLiteral(bc, append(o, []byte("https://"+localip+":"+strconv.Itoa(conf.Port)+"/esx/")...))

	logrus.WithFields(logrus.Fields{
		"file":  filepath,
	}).Info("uefi-https")

	return bc, nil
}