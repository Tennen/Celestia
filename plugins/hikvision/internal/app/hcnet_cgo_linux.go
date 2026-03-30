//go:build linux && hikvision_sdk

package app

/*
#cgo CFLAGS: -std=c11 -D_GNU_SOURCE -I${SRCDIR}/../../sdk/include
#cgo LDFLAGS: -ldl

#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "HCNetSDK.h"

typedef BOOL (*fn_NET_DVR_Init)(void);
typedef BOOL (*fn_NET_DVR_Cleanup)(void);
typedef BOOL (*fn_NET_DVR_SetSDKInitCfg)(int, void*);
typedef BOOL (*fn_NET_DVR_SetLogToFile)(DWORD, const char*, BOOL);
typedef LONG (*fn_NET_DVR_Login_V40)(NET_DVR_USER_LOGIN_INFO*, NET_DVR_DEVICEINFO_V40*);
typedef BOOL (*fn_NET_DVR_Logout)(LONG);
typedef DWORD (*fn_NET_DVR_GetLastError)(void);
typedef BOOL (*fn_NET_DVR_PTZControlWithSpeed_Other)(LONG, LONG, DWORD, DWORD, DWORD);
typedef LONG (*fn_NET_DVR_PlayBackByTime_V40)(LONG, NET_DVR_VOD_PARA*);
typedef BOOL (*fn_NET_DVR_PlayBackControl)(LONG, DWORD, DWORD, LONG*);
typedef BOOL (*fn_NET_DVR_StopPlayBack)(LONG);
typedef LONG (*fn_NET_DVR_FindFile_V40)(LONG, NET_DVR_FILECOND_V40*);
typedef LONG (*fn_NET_DVR_FindNextFile_V40)(LONG, NET_DVR_FINDDATA_V40*);
typedef BOOL (*fn_NET_DVR_FindClose_V30)(LONG);

enum {
    CEL_SDK_INIT_CFG_SDK_PATH = 2,
    CEL_SDK_INIT_CFG_LIBEAY_PATH = 3,
    CEL_SDK_INIT_CFG_SSLEAY_PATH = 4,
    CEL_PLAYSTART = 1,
    CEL_PLAYPAUSE = 3,
    CEL_PLAYSETPOS = 12,
    CEL_PLAYGETPOS = 13,
    CEL_FILE_SUCCESS = 1000,
    CEL_FILE_NOFIND = 1001,
    CEL_FILE_FINDING = 1002,
    CEL_FILE_NOMORE = 1003,
};

static void* g_sdk_handle = NULL;
static fn_NET_DVR_Init p_NET_DVR_Init = NULL;
static fn_NET_DVR_Cleanup p_NET_DVR_Cleanup = NULL;
static fn_NET_DVR_SetSDKInitCfg p_NET_DVR_SetSDKInitCfg = NULL;
static fn_NET_DVR_SetLogToFile p_NET_DVR_SetLogToFile = NULL;
static fn_NET_DVR_Login_V40 p_NET_DVR_Login_V40 = NULL;
static fn_NET_DVR_Logout p_NET_DVR_Logout = NULL;
static fn_NET_DVR_GetLastError p_NET_DVR_GetLastError = NULL;
static fn_NET_DVR_PTZControlWithSpeed_Other p_NET_DVR_PTZControlWithSpeed_Other = NULL;
static fn_NET_DVR_PlayBackByTime_V40 p_NET_DVR_PlayBackByTime_V40 = NULL;
static fn_NET_DVR_PlayBackControl p_NET_DVR_PlayBackControl = NULL;
static fn_NET_DVR_StopPlayBack p_NET_DVR_StopPlayBack = NULL;
static fn_NET_DVR_FindFile_V40 p_NET_DVR_FindFile_V40 = NULL;
static fn_NET_DVR_FindNextFile_V40 p_NET_DVR_FindNextFile_V40 = NULL;
static fn_NET_DVR_FindClose_V30 p_NET_DVR_FindClose_V30 = NULL;

static void cel_set_err(char* err, int err_len, const char* text) {
    if (!err || err_len <= 0) {
        return;
    }
    if (!text) {
        text = "unknown error";
    }
    snprintf(err, (size_t)err_len, "%s", text);
}

static int cel_load_symbol(void** out, const char* name, char* err, int err_len) {
    *out = dlsym(g_sdk_handle, name);
    if (!*out) {
        cel_set_err(err, err_len, dlerror());
        return 0;
    }
    return 1;
}

static DWORD cel_last_error(void) {
    if (!p_NET_DVR_GetLastError) {
        return 0;
    }
    return p_NET_DVR_GetLastError();
}

int cel_sdk_load(const char* lib_path, char* err, int err_len) {
    if (g_sdk_handle) {
        return 1;
    }
    g_sdk_handle = dlopen(lib_path, RTLD_NOW | RTLD_GLOBAL);
    if (!g_sdk_handle) {
        cel_set_err(err, err_len, dlerror());
        return 0;
    }

    if (!cel_load_symbol((void**)&p_NET_DVR_Init, "NET_DVR_Init", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_Cleanup, "NET_DVR_Cleanup", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_SetSDKInitCfg, "NET_DVR_SetSDKInitCfg", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_SetLogToFile, "NET_DVR_SetLogToFile", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_Login_V40, "NET_DVR_Login_V40", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_Logout, "NET_DVR_Logout", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_GetLastError, "NET_DVR_GetLastError", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_PTZControlWithSpeed_Other, "NET_DVR_PTZControlWithSpeed_Other", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_PlayBackByTime_V40, "NET_DVR_PlayBackByTime_V40", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_PlayBackControl, "NET_DVR_PlayBackControl", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_StopPlayBack, "NET_DVR_StopPlayBack", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_FindFile_V40, "NET_DVR_FindFile_V40", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_FindNextFile_V40, "NET_DVR_FindNextFile_V40", err, err_len)) return 0;
    if (!cel_load_symbol((void**)&p_NET_DVR_FindClose_V30, "NET_DVR_FindClose_V30", err, err_len)) return 0;
    return 1;
}

int cel_sdk_init(const char* sdk_dir, const char* crypto_path, const char* ssl_path, const char* log_dir, DWORD* out_err, char* err, int err_len) {
    if (!p_NET_DVR_SetSDKInitCfg || !p_NET_DVR_Init) {
        cel_set_err(err, err_len, "sdk symbols are not loaded");
        return 0;
    }

    NET_DVR_LOCAL_SDK_PATH sdk_path;
    memset(&sdk_path, 0, sizeof(sdk_path));
    if (sdk_dir) {
        size_t n = strlen(sdk_dir);
        if (n >= sizeof(sdk_path.sPath)) {
            n = sizeof(sdk_path.sPath) - 1;
        }
        memcpy(sdk_path.sPath, sdk_dir, n);
    }

    p_NET_DVR_SetSDKInitCfg(CEL_SDK_INIT_CFG_LIBEAY_PATH, (void*)crypto_path);
    p_NET_DVR_SetSDKInitCfg(CEL_SDK_INIT_CFG_SSLEAY_PATH, (void*)ssl_path);
    p_NET_DVR_SetSDKInitCfg(CEL_SDK_INIT_CFG_SDK_PATH, (void*)&sdk_path);

    if (!p_NET_DVR_Init()) {
        if (out_err) *out_err = cel_last_error();
        cel_set_err(err, err_len, "NET_DVR_Init failed");
        return 0;
    }
    if (log_dir && p_NET_DVR_SetLogToFile) {
        p_NET_DVR_SetLogToFile(3, log_dir, 0);
    }
    return 1;
}

void cel_sdk_cleanup(void) {
    if (p_NET_DVR_Cleanup) {
        p_NET_DVR_Cleanup();
    }
}

static void cel_fill_char(char* dst, size_t dst_len, const char* src) {
    memset(dst, 0, dst_len);
    if (!src || dst_len == 0) {
        return;
    }
    size_t n = strlen(src);
    if (n >= dst_len) {
        n = dst_len - 1;
    }
    memcpy(dst, src, n);
}

int cel_login(const char* host, int port, const char* username, const char* password, LONG* out_uid, int* out_start_chan, int* out_analog, int* out_digital_start, DWORD* out_err) {
    NET_DVR_USER_LOGIN_INFO login;
    NET_DVR_DEVICEINFO_V40 dev;
    memset(&login, 0, sizeof(login));
    memset(&dev, 0, sizeof(dev));

    cel_fill_char((char*)login.sDeviceAddress, sizeof(login.sDeviceAddress), host);
    cel_fill_char((char*)login.sUserName, sizeof(login.sUserName), username);
    cel_fill_char((char*)login.sPassword, sizeof(login.sPassword), password);
    login.wPort = (WORD)port;
    login.bUseAsynLogin = 0;
    login.byLoginMode = 0;
    login.byUseUTCTime = 0;

    LONG uid = p_NET_DVR_Login_V40(&login, &dev);
    if (uid < 0) {
        if (out_err) *out_err = cel_last_error();
        return 0;
    }
    if (out_uid) *out_uid = uid;
    if (out_start_chan) *out_start_chan = (int)dev.struDeviceV30.byStartChan;
    if (out_analog) *out_analog = (int)dev.struDeviceV30.byChanNum;
    if (out_digital_start) *out_digital_start = (int)dev.struDeviceV30.byStartDChan;
    return 1;
}

void cel_logout(LONG uid) {
    if (p_NET_DVR_Logout && uid >= 0) {
        p_NET_DVR_Logout(uid);
    }
}

int cel_ptz(LONG uid, LONG channel, DWORD cmd, DWORD stop, DWORD speed, DWORD* out_err) {
    if (!p_NET_DVR_PTZControlWithSpeed_Other(uid, channel, cmd, stop, speed)) {
        if (out_err) *out_err = cel_last_error();
        return 0;
    }
    return 1;
}

int cel_playback_open(LONG uid, LONG channel, NET_DVR_TIME begin, NET_DVR_TIME end, LONG* out_handle, DWORD* out_err) {
    NET_DVR_VOD_PARA vod;
    memset(&vod, 0, sizeof(vod));
    vod.dwSize = (DWORD)sizeof(vod);
    vod.struIDInfo.dwSize = (DWORD)sizeof(vod.struIDInfo);
    vod.struIDInfo.dwChannel = (DWORD)channel;
    vod.struBeginTime = begin;
    vod.struEndTime = end;

    LONG handle = p_NET_DVR_PlayBackByTime_V40(uid, &vod);
    if (handle < 0) {
        if (out_err) *out_err = cel_last_error();
        return 0;
    }
    if (out_handle) *out_handle = handle;
    return 1;
}

int cel_playback_control(LONG handle, DWORD command, DWORD value, LONG* out_value, DWORD* out_err) {
    LONG output = 0;
    if (!p_NET_DVR_PlayBackControl(handle, command, value, &output)) {
        if (out_err) *out_err = cel_last_error();
        return 0;
    }
    if (out_value) *out_value = output;
    return 1;
}

void cel_playback_stop(LONG handle) {
    if (p_NET_DVR_StopPlayBack && handle >= 0) {
        p_NET_DVR_StopPlayBack(handle);
    }
}

int cel_find_open(LONG uid, LONG channel, NET_DVR_TIME start, NET_DVR_TIME stop, LONG* out_handle, DWORD* out_err) {
    NET_DVR_FILECOND_V40 cond;
    memset(&cond, 0, sizeof(cond));
    cond.lChannel = channel;
    cond.dwFileType = 0xFF;
    cond.dwIsLocked = 0xFF;
    cond.dwUseCardNo = 0;
    cond.struStartTime = start;
    cond.struStopTime = stop;
    cond.byDrawFrame = 0;
    cond.byFindType = 0;
    cond.byQuickSearch = 0;
    cond.bySpecialFindInfoType = 0;
    cond.dwVolumeNum = 0;
    cond.byStreamType = 0;
    cond.byAudioFile = 0;

    LONG handle = p_NET_DVR_FindFile_V40(uid, &cond);
    if (handle < 0) {
        if (out_err) *out_err = cel_last_error();
        return 0;
    }
    if (out_handle) *out_handle = handle;
    return 1;
}

LONG cel_find_next(LONG handle, NET_DVR_FINDDATA_V40* out_data, DWORD* out_err) {
    LONG ret = p_NET_DVR_FindNextFile_V40(handle, out_data);
    if (ret < 0 && out_err) {
        *out_err = cel_last_error();
    }
    return ret;
}

void cel_find_close(LONG handle) {
    if (p_NET_DVR_FindClose_V30 && handle >= 0) {
        p_NET_DVR_FindClose_V30(handle);
    }
}
*/
import "C"

import (
	"fmt"
	"strings"
	"time"
	"unsafe"
)

const (
	sdkPlayStart  = uint32(C.CEL_PLAYSTART)
	sdkPlayPause  = uint32(C.CEL_PLAYPAUSE)
	sdkPlaySetPos = uint32(C.CEL_PLAYSETPOS)
	sdkPlayGetPos = uint32(C.CEL_PLAYGETPOS)

	sdkFindSuccess = int(C.CEL_FILE_SUCCESS)
	sdkFindNoFile  = int(C.CEL_FILE_NOFIND)
	sdkFindFinding = int(C.CEL_FILE_FINDING)
	sdkFindNoMore  = int(C.CEL_FILE_NOMORE)
)

type sdkLoginResult struct {
	UserID              int
	StartChannel        int
	AnalogChannels      int
	DigitalStartChannel int
}

type sdkFindData struct {
	FileName   string
	Start      time.Time
	End        time.Time
	FileSize   uint32
	FileType   uint8
	Locked     bool
	FileIndex  uint32
	StreamType uint8
}

func sdkToTime(value time.Time) C.NET_DVR_TIME {
	value = value.In(time.Local)
	return C.NET_DVR_TIME{
		dwYear:   C.DWORD(value.Year()),
		dwMonth:  C.DWORD(int(value.Month())),
		dwDay:    C.DWORD(value.Day()),
		dwHour:   C.DWORD(value.Hour()),
		dwMinute: C.DWORD(value.Minute()),
		dwSecond: C.DWORD(value.Second()),
	}
}

func sdkTimeFromC(value C.NET_DVR_TIME) time.Time {
	year := int(value.dwYear)
	if year <= 0 {
		year = 1970
	}
	month := time.Month(clampInt(int(value.dwMonth), 1, 12))
	day := clampInt(int(value.dwDay), 1, 31)
	hour := clampInt(int(value.dwHour), 0, 23)
	minute := clampInt(int(value.dwMinute), 0, 59)
	second := clampInt(int(value.dwSecond), 0, 59)
	return time.Date(year, month, day, hour, minute, second, 0, time.Local)
}

func sdkLoad(libPath string) error {
	cPath := C.CString(libPath)
	defer C.free(unsafe.Pointer(cPath))
	errBuf := (*C.char)(C.malloc(1024))
	defer C.free(unsafe.Pointer(errBuf))
	if C.cel_sdk_load(cPath, errBuf, 1024) == 0 {
		return fmt.Errorf("load sdk failed: %s", C.GoString(errBuf))
	}
	return nil
}

func sdkInit(sdkDir, cryptoPath, sslPath, logDir string) error {
	cDir := C.CString(sdkDir)
	defer C.free(unsafe.Pointer(cDir))
	cCrypto := C.CString(cryptoPath)
	defer C.free(unsafe.Pointer(cCrypto))
	cSSL := C.CString(sslPath)
	defer C.free(unsafe.Pointer(cSSL))
	var cLog *C.char
	if strings.TrimSpace(logDir) != "" {
		cLog = C.CString(logDir)
		defer C.free(unsafe.Pointer(cLog))
	}

	errBuf := (*C.char)(C.malloc(1024))
	defer C.free(unsafe.Pointer(errBuf))
	var sdkErr C.DWORD
	if C.cel_sdk_init(cDir, cCrypto, cSSL, cLog, &sdkErr, errBuf, 1024) == 0 {
		return fmt.Errorf("NET_DVR_Init failed: %s (error_code=%d)", C.GoString(errBuf), uint32(sdkErr))
	}
	return nil
}

func sdkCleanup() {
	C.cel_sdk_cleanup()
}

func sdkLogin(host string, port int, username, password string) (sdkLoginResult, error) {
	cHost := C.CString(host)
	defer C.free(unsafe.Pointer(cHost))
	cUser := C.CString(username)
	defer C.free(unsafe.Pointer(cUser))
	cPass := C.CString(password)
	defer C.free(unsafe.Pointer(cPass))
	var uid C.LONG
	var startChan C.int
	var analog C.int
	var digital C.int
	var sdkErr C.DWORD
	if C.cel_login(cHost, C.int(port), cUser, cPass, &uid, &startChan, &analog, &digital, &sdkErr) == 0 {
		return sdkLoginResult{}, fmt.Errorf("NET_DVR_Login_V40 failed (error_code=%d)", uint32(sdkErr))
	}
	return sdkLoginResult{
		UserID:              int(uid),
		StartChannel:        int(startChan),
		AnalogChannels:      int(analog),
		DigitalStartChannel: int(digital),
	}, nil
}

func sdkLogout(userID int) {
	C.cel_logout(C.LONG(userID))
}

func sdkPTZ(userID, channel int, cmd uint32, stop bool, speed int) error {
	stopValue := uint32(0)
	if stop {
		stopValue = 1
	}
	var sdkErr C.DWORD
	if C.cel_ptz(C.LONG(userID), C.LONG(channel), C.DWORD(cmd), C.DWORD(stopValue), C.DWORD(speed), &sdkErr) == 0 {
		return fmt.Errorf("PTZ control failed (error_code=%d)", uint32(sdkErr))
	}
	return nil
}

func sdkPlaybackOpen(userID, channel int, start, end time.Time) (int, error) {
	begin := sdkToTime(start)
	finish := sdkToTime(end)
	var handle C.LONG
	var sdkErr C.DWORD
	if C.cel_playback_open(C.LONG(userID), C.LONG(channel), begin, finish, &handle, &sdkErr) == 0 {
		return -1, fmt.Errorf("NET_DVR_PlayBackByTime_V40 failed (error_code=%d)", uint32(sdkErr))
	}
	if _, err := sdkPlaybackControl(int(handle), sdkPlayStart, 0); err != nil {
		sdkPlaybackStop(int(handle))
		return -1, err
	}
	return int(handle), nil
}

func sdkPlaybackControl(handle int, command uint32, value uint32) (int, error) {
	var out C.LONG
	var sdkErr C.DWORD
	if C.cel_playback_control(C.LONG(handle), C.DWORD(command), C.DWORD(value), &out, &sdkErr) == 0 {
		return 0, fmt.Errorf("NET_DVR_PlayBackControl failed (error_code=%d)", uint32(sdkErr))
	}
	return int(out), nil
}

func sdkPlaybackStop(handle int) {
	C.cel_playback_stop(C.LONG(handle))
}

func sdkFindOpen(userID, channel int, start, end time.Time) (int, error) {
	begin := sdkToTime(start)
	finish := sdkToTime(end)
	var handle C.LONG
	var sdkErr C.DWORD
	if C.cel_find_open(C.LONG(userID), C.LONG(channel), begin, finish, &handle, &sdkErr) == 0 {
		return -1, fmt.Errorf("NET_DVR_FindFile_V40 failed (error_code=%d)", uint32(sdkErr))
	}
	return int(handle), nil
}

func sdkFindNext(handle int) (int, sdkFindData, error) {
	var data C.NET_DVR_FINDDATA_V40
	var sdkErr C.DWORD
	ret := int(C.cel_find_next(C.LONG(handle), &data, &sdkErr))
	if ret < 0 {
		return ret, sdkFindData{}, fmt.Errorf("NET_DVR_FindNextFile_V40 failed (error_code=%d)", uint32(sdkErr))
	}
	if ret != sdkFindSuccess {
		return ret, sdkFindData{}, nil
	}
	fileName := strings.TrimSpace(C.GoString((*C.char)(unsafe.Pointer(&data.sFileName[0]))))
	return ret, sdkFindData{
		FileName:   fileName,
		Start:      sdkTimeFromC(data.struStartTime),
		End:        sdkTimeFromC(data.struStopTime),
		FileSize:   uint32(data.dwFileSize),
		FileType:   uint8(data.byFileType),
		Locked:     data.byLocked != 0,
		FileIndex:  uint32(data.dwFileIndex),
		StreamType: uint8(data.byStreamType),
	}, nil
}

func sdkFindClose(handle int) {
	C.cel_find_close(C.LONG(handle))
}

func clampInt(v, minValue, maxValue int) int {
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
}
