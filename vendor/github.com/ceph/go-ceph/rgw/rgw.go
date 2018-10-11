package rgw

/*
#cgo LDFLAGS: -lrgw
#cgo CPPFLAGS: -D_FILE_OFFSET_BITS=64
#include <stdlib.h>
#include <rados/librgw.h>
#include <rados/rgw_file.h>
*/
import "C"
import "fmt"
import "unsafe"
import "strings"

type CephError int

func (e CephError) Error() string {
	return fmt.Sprintf("rgw: ret=%d", e)
}

func Create(args []string) C.librgw_t {
	var rgw C.librgw_t

	argv := make([](*C.char), 0)
	for i, _ := range args {
		cs := C.CString(args[i])
		defer C.free(unsafe.Pointer(cs))
		csptr := (*C.char)(unsafe.Pointer(cs))
		argv = append(argv, csptr)
	}
	argc := C.int(len(args))
	C.librgw_create(&rgw, argc, (**C.char)(unsafe.Pointer(&argv[0])))
	return rgw
}

func Shutdown(rgw C.librgw_t) {
	C.librgw_shutdown(rgw)
}

func Mount(rgw C.librgw_t, uid, key, secret string) (*C.struct_rgw_fs, error) {
	var rgwfs *C.struct_rgw_fs
	ret := C.rgw_mount2(rgw, C.CString(uid), C.CString(key), C.CString(secret), C.CString("/"), &rgwfs, C.RGW_MOUNT_FLAG_NONE)
	if ret != 0 {
		return nil, CephError(ret)
	}
	return rgwfs, nil
}

func Umount(rgwfs *C.struct_rgw_fs) {
	C.rgw_umount(rgwfs, C.RGW_UMOUNT_FLAG_NONE)
}

type RgwFileHandle struct {
	rgwfs    *C.struct_rgw_fs
	instance *C.struct_rgw_file_handle
}

func MakeRgwFileHandle(rgwfs *C.struct_rgw_fs) RgwFileHandle {
	return RgwFileHandle{rgwfs: rgwfs, instance: nil}
}

func (fh RgwFileHandle) Lookup(path string) (RgwFileHandle, error) {
	handle, err := Lookup(fh.rgwfs, fh.instance, path)
	return RgwFileHandle{rgwfs: fh.rgwfs, instance: handle}, err
}

func (fh RgwFileHandle) Release() {
	if fh.instance != nil {
		Release(fh.rgwfs, fh.instance)
	}
}

func (fh RgwFileHandle) GetAttr() Attr {
	attr, _ := Getattr(fh.rgwfs, fh.instance)
	return attr
}

func (fh RgwFileHandle) SetAttr(attrMap map[string]uint) Attr {
	attr, _ := Setattr(fh.rgwfs, fh.instance, attrMap)
	return attr
}

func Lookup(rgwfs *C.struct_rgw_fs, fh *C.struct_rgw_file_handle, path string) (*C.struct_rgw_file_handle, error) {
	if fh == nil {
		fh = rgwfs.root_fh
	}
	var handle *C.struct_rgw_file_handle
	ret := C.rgw_lookup(rgwfs, fh, C.CString(path), &handle, C.RGW_LOOKUP_FLAG_RCB)
	if ret != 0 {
		return nil, CephError(ret)
	}
	return handle, nil
}

func Release(rgwfs *C.struct_rgw_fs, handle *C.struct_rgw_file_handle) error {
	ret := C.rgw_fh_rele(rgwfs, handle, C.RGW_FH_RELE_FLAG_NONE)
	if ret != 0 {
		return CephError(ret)
	}
	return nil
}

type Attr struct {
	Uid  uint
	Gid  uint
	Mode uint
}

func (a Attr) IsDir() bool {
	return (a.Mode & 0170000) == 0040000
}

func (a Attr) GetUgoMode() uint {
	return a.Mode & 0777
}

func (a Attr) IsInitialized() bool {
	if a.IsDir() {
		return !(a.Uid == 0 && a.Gid == 0 && a.GetUgoMode() == 0777)
	}
	return !(a.Uid == 0 && a.Gid == 0 && a.GetUgoMode() == 0666)
}

func Getattr(rgwfs *C.struct_rgw_fs, handle *C.struct_rgw_file_handle) (Attr, error) {
	var stat C.struct_stat
	ret := C.rgw_getattr(rgwfs, handle, &stat, C.RGW_GETATTR_FLAG_NONE)
	if ret != 0 {
		return Attr{}, CephError(ret)
	}
	attr := Attr{Uid: uint(stat.st_uid), Gid: uint(stat.st_gid), Mode: uint(stat.st_mode)}
	return attr, nil
}

func Setattr(rgwfs *C.struct_rgw_fs, handle *C.struct_rgw_file_handle, attrMap map[string]uint) (Attr, error) {
	var stat C.struct_stat
	mask := C.uint32_t(0)

	for key := range attrMap {
		switch strings.ToLower(key) {
		case "uid":
			mask = mask | C.RGW_SETATTR_UID
			stat.st_uid = C.uid_t(attrMap[key])
		case "gid":
			mask = mask | C.RGW_SETATTR_GID
			stat.st_gid = C.gid_t(attrMap[key])
		case "mode":
			mask = mask | C.RGW_SETATTR_MODE
			stat.st_mode = C.mode_t(attrMap[key])
		}
	}
	ret := C.rgw_setattr(rgwfs, handle, &stat, mask, C.RGW_SETATTR_FLAG_NONE)
	if ret != 0 {
		return Attr{}, CephError(ret)
	}
	return Attr{}, nil
}
