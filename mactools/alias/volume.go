package alias

import "C"
import (
	"errors"
	"unsafe"
)

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreFoundation -framework CoreServices
#import <CoreFoundation/CoreFoundation.h>
#import <CoreServices/CoreServices.h>

const char* OSErrDescription(OSErr err) {
    switch (err) {
        case nsvErr: return "Volume not found";
        case ioErr: return "I/O error.";
        case bdNamErr: return "Bad filename or volume name.";
        case mFulErr: return "Memory full (open) or file won't fit (load)";
        case tmfoErr: return "Too many files open.";
        case fnfErr: return "File or directory not found; incomplete pathname.";
        case volOffLinErr: return "Volume is offline.";
        case nsDrvErr: return "No such drive.";
        case dirNFErr: return "Directory not found or incomplete pathname.";
        case tmwdoErr: return "Too many working directories open.";
    }
    return "Could not get volume name";
}

char* GetVolumeName(const char* path) {
    CFStringRef volumePath = CFStringCreateWithCString(NULL, path, kCFStringEncodingUTF8);
    CFURLRef url = CFURLCreateWithFileSystemPath(NULL, volumePath, kCFURLPOSIXPathStyle, true);

    FSRef urlFS;
    if (!CFURLGetFSRef(url, &urlFS)) {
        return "Failed to convert URL to file or directory object";
    }

    FSCatalogInfo urlInfo;
    OSErr err = FSGetCatalogInfo(&urlFS, kFSCatInfoVolume, &urlInfo, NULL, NULL, NULL);
    if (err != noErr) {
        return (char*)OSErrDescription(err);
    }

    HFSUniStr255 outString;
    err = FSGetVolumeInfo(urlInfo.volume, 0, NULL, kFSVolInfoNone, NULL, &outString, NULL);
    if (err != noErr) {
        return (char*)OSErrDescription(err);
    }

    CFStringRef cfStr = CFStringCreateWithCharacters(NULL, outString.unicode, outString.length);
    CFIndex bufferSize = CFStringGetMaximumSizeForEncoding(CFStringGetLength(cfStr), kCFStringEncodingUTF8) + 1;
    char* result = (char*)malloc(bufferSize);
    if (CFStringGetCString(cfStr, result, bufferSize, kCFStringEncodingUTF8)) {
        CFRelease(cfStr);
        CFRelease(url);
        CFRelease(volumePath);
        return result;
    }

    free(result);
    CFRelease(cfStr);
    CFRelease(url);
    CFRelease(volumePath);
    return "Failed to convert volume name to string";
}
*/
import "C"

func GetVolumeName(path string) (string, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	cResult := C.GetVolumeName(cPath)
	defer C.free(unsafe.Pointer(cResult))

	result := C.GoString(cResult)

	if result == "Failed to convert URL to file or directory object" ||
		result == "Failed to convert volume name to string" {
		return "", errors.New(result)
	}

	return result, nil
}
