#ifndef CELESTIA_HCNET_TYPES_COMPAT_H
#define CELESTIA_HCNET_TYPES_COMPAT_H

#include <stdint.h>

typedef int BOOL;
typedef uint32_t DWORD;
typedef uint16_t WORD;
typedef int32_t LONG;
typedef uint8_t BYTE;
typedef void* LPVOID;

#define NET_SDK_MAX_FILE_PATH 256
#define NET_DVR_DEV_ADDRESS_MAX_LEN 129
#define NET_DVR_LOGIN_USERNAME_MAX_LEN 64
#define NET_DVR_LOGIN_PASSWD_MAX_LEN 64
#define SERIALNO_LEN 48
#define STREAM_ID_LEN 32
#define CARDNUM_LEN_OUT 32
#define GUID_LEN 16
#define NET_DVR_FILE_NAME_LEN 100
#define SPECIAL_FIND_INFO_LEN 8

#pragma pack(push,1)

typedef struct {
    DWORD dwYear;
    DWORD dwMonth;
    DWORD dwDay;
    DWORD dwHour;
    DWORD dwMinute;
    DWORD dwSecond;
} NET_DVR_TIME;

typedef struct {
    BYTE sSerialNumber[SERIALNO_LEN];
    BYTE byAlarmInPortNum;
    BYTE byAlarmOutPortNum;
    BYTE byDiskNum;
    BYTE byDVRType;
    BYTE byChanNum;
    BYTE byStartChan;
    BYTE byAudioChanNum;
    BYTE byIPChanNum;
    BYTE byZeroChanNum;
    BYTE byMainProto;
    BYTE bySubProto;
    BYTE bySupport;
    BYTE bySupport1;
    BYTE bySupport2;
    WORD wDevType;
    BYTE bySupport3;
    BYTE byMultiStreamProto;
    BYTE byStartDChan;
    BYTE byStartDTalkChan;
    BYTE byHighDChanNum;
    BYTE bySupport4;
    BYTE byLanguageType;
    BYTE byVoiceInChanNum;
    BYTE byStartVoiceInChanNo;
    BYTE bySupport5;
    BYTE bySupport6;
    BYTE byMirrorChanNum;
    WORD wStartMirrorChanNo;
    BYTE bySupport7;
    BYTE byRes2;
} NET_DVR_DEVICEINFO_V30;

typedef struct {
    NET_DVR_DEVICEINFO_V30 struDeviceV30;
    BYTE bySupportLock;
    BYTE byRetryLoginTime;
    BYTE byPasswordLevel;
    BYTE byRes1;
    DWORD dwSurplusLockTime;
    BYTE byCharEncodeType;
    BYTE bySupportDev5;
    BYTE bySupport;
    BYTE byLoginMode;
    DWORD dwOEMCode;
    int32_t iResidualValidity;
    BYTE byResidualValidity;
    BYTE bySingleStartDTalkChan;
    BYTE bySingleDTalkChanNums;
    BYTE byPassWordResetLevel;
    BYTE bySupportStreamEncrypt;
    BYTE byMarketType;
    BYTE byRes2[238];
} NET_DVR_DEVICEINFO_V40;

typedef struct {
    BYTE sDeviceAddress[NET_DVR_DEV_ADDRESS_MAX_LEN];
    BYTE byUseTransport;
    WORD wPort;
    BYTE sUserName[NET_DVR_LOGIN_USERNAME_MAX_LEN];
    BYTE sPassword[NET_DVR_LOGIN_PASSWD_MAX_LEN];
    LPVOID cbLoginResult;
    LPVOID pUser;
    DWORD bUseAsynLogin;
    BYTE byProxyType;
    BYTE byUseUTCTime;
    BYTE byLoginMode;
    BYTE byHttps;
    DWORD iProxyID;
    BYTE byVerifyMode;
    BYTE byRes2[119];
} NET_DVR_USER_LOGIN_INFO;

typedef struct {
    BYTE sPath[NET_SDK_MAX_FILE_PATH];
    BYTE byRes[128];
} NET_DVR_LOCAL_SDK_PATH;

typedef struct {
    DWORD dwSize;
    BYTE byID[STREAM_ID_LEN];
    DWORD dwChannel;
    BYTE byRes[32];
} NET_DVR_STREAM_INFO;

typedef struct {
    DWORD dwSize;
    NET_DVR_STREAM_INFO struIDInfo;
    NET_DVR_TIME struBeginTime;
    NET_DVR_TIME struEndTime;
    LPVOID hWnd;
    BYTE byDrawFrame;
    BYTE byVolumeType;
    BYTE byVolumeNum;
    BYTE byStreamType;
    DWORD dwFileIndex;
    BYTE byAudioFile;
    BYTE byCourseFile;
    BYTE byDownload;
    BYTE byOptimalStreamType;
    BYTE byRes2[20];
} NET_DVR_VOD_PARA;

typedef struct {
    LONG lChannel;
    DWORD dwFileType;
    DWORD dwIsLocked;
    DWORD dwUseCardNo;
    BYTE sCardNumber[CARDNUM_LEN_OUT];
    NET_DVR_TIME struStartTime;
    NET_DVR_TIME struStopTime;
    BYTE byDrawFrame;
    BYTE byFindType;
    BYTE byQuickSearch;
    BYTE bySpecialFindInfoType;
    DWORD dwVolumeNum;
    BYTE byWorkingDeviceGUID[GUID_LEN];
    BYTE uSpecialFindInfo[SPECIAL_FIND_INFO_LEN];
    BYTE byStreamType;
    BYTE byAudioFile;
    BYTE byRes2[30];
} NET_DVR_FILECOND_V40;

typedef struct {
    BYTE sFileName[NET_DVR_FILE_NAME_LEN];
    NET_DVR_TIME struStartTime;
    NET_DVR_TIME struStopTime;
    DWORD dwFileSize;
    BYTE sCardNum[CARDNUM_LEN_OUT];
    BYTE byLocked;
    BYTE byFileType;
    BYTE byQuickSearch;
    BYTE byRes;
    DWORD dwFileIndex;
    BYTE byStreamType;
    BYTE byRes1[127];
} NET_DVR_FINDDATA_V40;

#pragma pack(pop)

#endif
