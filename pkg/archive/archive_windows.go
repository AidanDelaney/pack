//go:build windows

package archive

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

func hasHardlinks(fi os.FileInfo, path string) (bool, error) {
	var numberOfLinks uint32
	switch v := fi.Sys().(type) {
	case *syscall.ByHandleFileInformation:
		numberOfLinks = v.NumberOfLinks
	default:
		// We need an instance of a ByHandleFileInformation to read NumberOfLinks
		info, err := open(path)
		if err != nil {
			return false, err
		}
		numberOfLinks = info.NumberOfLinks
	}
	return numberOfLinks > 1, nil
}

func getInodeFromStat(stat interface{}, path string) (inode uint64, err error) {
	s, ok := stat.(*syscall.ByHandleFileInformation)
	if ok {
		inode = (uint64(s.FileIndexHigh) << 32) | uint64(s.FileIndexLow)
	} else {
		s, err = open(path)
		if err == nil {
			inode = (uint64(s.FileIndexHigh) << 32) | uint64(s.FileIndexLow)
		}
	}
	return
}

func open(path string) (*syscall.ByHandleFileInformation, error) {
	fPath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	handle, err := syscall.CreateFile(
		fPath,
		windows.FILE_READ_ATTRIBUTES,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_BACKUP_SEMANTICS,
		0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(handle)

	var info syscall.ByHandleFileInformation
	err = syscall.GetFileInformationByHandle(handle, &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}
