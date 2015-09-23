/*******************************************************************************
*
*   Copyright 2015 Stefan Majewsky <majewsky@gmx.net>
*
*   This program is free software; you can redistribute it and/or modify it
*   under the terms of the GNU General Public License as published by the Free
*   Software Foundation; either version 2 of the License, or (at your option)
*   any later version.
*
*   This program is distributed in the hope that it will be useful, but WITHOUT
*   ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
*   FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public License for
*   more details.
*
*   You should have received a copy of the GNU General Public License along
*   with this program; if not, write to the Free Software Foundation, Inc.,
*   51 Franklin Street, Fifth Floor, Boston, MA 02110-1301, USA.
*
********************************************************************************/

package common

import (
	"io/ioutil"
	"os"
	"syscall"
)

//IsManageableFile returns whether the file can be managed by Holo (i.e. is a
//regular file or a symlink).
func IsManageableFile(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() || IsFileInfoASymbolicLink(info)
}

//IsFileInfoASymbolicLink returns whether the given FileInfo describes a symlink.
func IsFileInfoASymbolicLink(fileInfo os.FileInfo) bool {
	return (fileInfo.Mode() & os.ModeType) == os.ModeSymlink
}

//IsNewerThan returns true if the first file is newer than the second file.
func IsNewerThan(path1, path2 string) (bool, error) {
	info1, err := os.Lstat(path1)
	if err != nil {
		return false, err
	}
	info2, err := os.Lstat(path2)
	if err != nil {
		return false, err
	}

	//Usually, we rely on the mtime to tell if the file path1 has been modified
	//after being created from the file at path2 (see copyFileImpl). This relies
	//on manually applying the mtime from path2 to path1 in ApplyFilePermissions.
	//But since Unix does not allow to update the mtime on symlinks, ignore the
	//mtime of symlinks.
	if IsFileInfoASymbolicLink(info1) {
		return false, nil
	}

	return info1.ModTime().After(info2.ModTime()), nil
}

//CopyFile copies a regular file or symlink, including the file metadata.
func CopyFile(fromPath, toPath string) error {
	info, err := os.Lstat(fromPath)
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		return copyFileImpl(fromPath, toPath)
	}
	return copySymlinkImpl(fromPath, toPath)
}

func copyFileImpl(fromPath, toPath string) error {
	//copy contents
	data, err := ioutil.ReadFile(fromPath)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(toPath, data, 0600)
	if err != nil {
		return err
	}

	return ApplyFilePermissions(fromPath, toPath)
}

func copySymlinkImpl(fromPath, toPath string) error {
	//read link target
	target, err := os.Readlink(fromPath)
	if err != nil {
		return err
	}
	//remove old file or link if it exists
	if IsManageableFile(toPath) {
		err = os.Remove(toPath)
		if err != nil {
			return err
		}
	}
	//create new link
	err = os.Symlink(target, toPath)
	if err != nil {
		return err
	}

	return nil
}

//ApplyFilePermissions applies various metadata (permissions, ownership,
//timestamps) from the first file to the second file.
//
//TODO: This should be called CopyFileMetadata instead.
func ApplyFilePermissions(fromPath, toPath string) error {
	//apply permissions, ownership, modification date from source file to target file
	//NOTE: We cannot just pass the FileMode in WriteFile(), because its
	//FileMode argument is only applied when a new file is created, not when
	//an existing one is truncated.
	info, err := os.Lstat(fromPath)
	if err != nil {
		return err
	}
	targetInfo, err := os.Lstat(toPath)
	if err != nil {
		return err
	}

	if !IsFileInfoASymbolicLink(targetInfo) {
		//apply permissions
		err = os.Chmod(toPath, info.Mode())
		if err != nil {
			return err
		}

		//apply ownership
		stat := info.Sys().(*syscall.Stat_t) // UGLY
		err = os.Chown(toPath, int(stat.Uid), int(stat.Gid))
		if err != nil {
			return err
		}

		//apply timestamps
		err = os.Chtimes(toPath, info.ModTime(), info.ModTime())
		if err != nil {
			return err
		}
	}

	return nil
}