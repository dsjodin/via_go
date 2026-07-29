/*
Copyright (c) 2015 VMware, Inc. All Rights Reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"io"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dsjodin/via_go/config"
	"github.com/dsjodin/via_go/db"
	"github.com/dsjodin/via_go/internal/boot"
	"github.com/dsjodin/via_go/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"

	"github.com/pin/tftp"
)

func readHandler(conf *config.Config) func(string, io.ReaderFrom) error {
	return func(filename string, rf io.ReaderFrom) error {

		// get the requesting ip-address and our source address
		raddr := rf.(tftp.OutgoingTransfer).RemoteAddr()
		laddr := rf.(tftp.RequestPacketInfo).LocalIP()

		//strip the port
		ip, _, _ := net.SplitHostPort(raddr.String())

		//get the object that correlates with the ip
		var host models.Host
		db.DB.Preload(clause.Associations).First(&host, "ip = ?", ip)

		//get the image info that correlates with the host
		var image models.Image
		db.DB.First(&image, "id = ?", host.Group.ImageID)

		logrus.WithFields(logrus.Fields{
			"raddr":    raddr,
			"laddr":    laddr,
			"filename": filename,
			"imageid":  image.ID,
			"hostid":   host.ID,
		}).Debug("tftpd")

		//if the filename is mboot.efi, we hijack it and serve the mboot.efi file that is part of that specific image, this guarantees that you always get an mboot file that works for the build
		switch filename {
		case "mboot.efi":
			logrus.WithFields(logrus.Fields{
				ip: "requesting mboot.efi",
			}).Info("tftpd")
			logrus.WithFields(logrus.Fields{
				"id":           host.ID,
				"percentage":   10,
				"progresstext": "mboot.efi",
			}).Info("progress")
			mboot, err := boot.Mboot(image.Path)
			if err != nil {
				return err
			}
			filename = mboot
			host.Progress = 10
			host.Progresstext = "mboot.efi"
			db.DB.Save(&host)
		case "crypto64.efi":
			logrus.WithFields(logrus.Fields{
				ip: "requesting crypto64.efi",
			}).Info("tftpd")
			logrus.WithFields(logrus.Fields{
				"id":           host.ID,
				"percentage":   12,
				"progresstext": "crypto64.efi",
			}).Info("progress")
			crypto, err := boot.Crypto64(image.Path)
			if err != nil {
				return err
			}
			filename = crypto
			host.Progress = 12
			host.Progresstext = "crypto64.efi"
			db.DB.Save(&host)
		case "boot.cfg", "/boot.cfg":
			// serveBootCfg writes the response itself, so return rather than
			// falling through to the file handling below — which used to stat
			// "boot.cfg" in the working directory and return its error even
			// though the transfer had already succeeded.
			return serveBootCfg(filename, host, image, rf, conf)
		default:
			//if no case matches, chroot to /images
			if _, err := os.Stat("images/" + filename); err == nil {
				filename = "images/" + filename
				logrus.WithFields(logrus.Fields{
					"lowercase file": filename,
				}).Debug("tftpd")
			} else {
				dir, file := path.Split(filename)
				upperfile := strings.ToUpper(string(file))
				filename = "images/" + dir + upperfile
				logrus.WithFields(logrus.Fields{
					"uppercase file": filename,
				}).Debug("tftpd")
			}
		}

		// get the filesize to send filelength
		fi, err := os.Stat(filename)
		if err != nil {
			return err
		}

		//set the filesize so that its advertized.
		rf.(tftp.OutgoingTransfer).SetSize(fi.Size())

		file, err := os.Open(filename)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"could not open file": err,
			}).Debug("tftpd")
			return err
		}
		n, err := rf.ReadFrom(file)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"could not read from file": err,
			}).Debug("tftpd")
			return err
		}
		logrus.WithFields(logrus.Fields{
			"id":    host.ID,
			"ip":    host.IP,
			"host":  host.Hostname,
			"file":  filename,
			"bytes": n,
		}).Info("tftpd")
		return nil
	}
}

func TFTPd(conf *config.Config) {
	s := tftp.NewServer(readHandler(conf), nil)
	s.SetTimeout(5 * time.Second)  // optional
	err := s.ListenAndServe(":69") // blocks until s.Shutdown() is called
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"could not start tftp server:": err,
		}).Info("tftpd")
		os.Exit(1)
	}
}

func serveBootCfg(filename string, host models.Host, image models.Image, rf io.ReaderFrom, conf *config.Config) error {
	var group models.Group
	db.DB.Preload(clause.Associations).First(&group, "id = ?", host.GroupID)

	// get the requesting ip-address and our source address
	raddr := rf.(tftp.OutgoingTransfer).RemoteAddr()
	laddr := rf.(tftp.RequestPacketInfo).LocalIP()

	//strip the port
	ip, _, _ := net.SplitHostPort(raddr.String())

	logrus.WithFields(logrus.Fields{
		ip: "requesting boot.cfg",
	}).Info("tftpd")
	logrus.WithFields(logrus.Fields{
		"id":           host.ID,
		"percentage":   15,
		"progresstext": "installation",
	}).Info("progress")
	host.Progress = 15
	host.Progresstext = "installation"
	db.DB.Save(&host)

	path, err := boot.Config(image.Path)
	if err != nil {
		logrus.Warn(err)
		return err
	}

	bc, err := os.ReadFile(path)
	if err != nil {
		logrus.Warn(err)
		return err
	}

	out := boot.RenderConfig(bc, boot.Params{
		// TFTP serves a flat directory, so the loader fetches the rest of the
		// image by folder name rather than by URL.
		Prefix:       boot.TFTPPrefix(image.Path),
		KickstartURL: boot.KickstartURL(laddr.String(), conf.Port),
		Mac:          host.Mac,
		IP:           host.IP,
		Netmask:      group.Netmask,
		Gateway:      group.Gateway,
		Vlan:         host.Group.Vlan,
		// Matches --forceunsupportedinstall in the kickstart template.
		AllowLegacyCPU: true,
	})

	// Send the data from the buffer to the client
	buff := bytes.NewBuffer(out)
	rf.(tftp.OutgoingTransfer).SetSize(int64(buff.Len()))
	n, err := rf.ReadFrom(buff)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"os.Stderr": err,
		}).Debug("tftpd")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"file":  filename,
		"bytes": n,
	}).Info("tftpd")

	return nil
}
