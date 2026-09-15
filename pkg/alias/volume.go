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
char* GetVolumeNameV2(const char* path) {
    CFStringRef volumePath = CFStringCreateWithCString(NULL, path, kCFStringEncodingUTF8);
    if (volumePath == NULL) {
        return "Failed to create CFString from path";
    }

    CFURLRef url = CFURLCreateWithFileSystemPath(NULL, volumePath, kCFURLPOSIXPathStyle, true);
    CFRelease(volumePath);
    if (url == NULL) {
        return "Failed to create CFURL from path";
    }

    CFStringRef volumeName = NULL;
    Boolean success = CFURLCopyResourcePropertyForKey(url, kCFURLVolumeNameKey, &volumeName, NULL);
    CFRelease(url);

    if (!success || volumeName == NULL) {
        return "Failed to get volume name";
    }

    CFIndex bufferSize = CFStringGetMaximumSizeForEncoding(CFStringGetLength(volumeName), kCFStringEncodingUTF8) + 1;
    char* result = (char*)malloc(bufferSize);
    if (result == NULL) {
        CFRelease(volumeName);
        return "Failed to allocate memory for volume name";
    }

    if (CFStringGetCString(volumeName, result, bufferSize, kCFStringEncodingUTF8)) {
        CFRelease(volumeName);
        return result;
    }

    free(result);
    CFRelease(volumeName);
    return "Failed to convert volume name to string";
}
*/
import "C"

func GetVolumeName(path string) (string, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	cResult := C.GetVolumeNameV2(cPath)
	defer C.free(unsafe.Pointer(cResult))

	result := C.GoString(cResult)

	if result == "Failed to create CFString from path" ||
		result == "Failed to create CFURL from path" ||
		result == "Failed to get volume name" ||
		result == "Failed to allocate memory for volume name" ||
		result == "Failed to convert volume name to string" {
		return "", errors.New(result)
	}

	return result, nil
}
