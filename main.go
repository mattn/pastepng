package main

import (
	"flag"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/cwchiu/go-winapi"
)

var base = "clipboard"

var (
	modkernel32    = syscall.NewLazyDLL("kernel32.dll")
	procGlobalSize = modkernel32.NewProc("GlobalSize")

	modgdipluls                    = syscall.NewLazyDLL("gdiplus.dll")
	procGdipCreateBitmapFromGdiDib = modgdipluls.NewProc("GdipCreateBitmapFromGdiDib")
	procGdipSaveImageToFile        = modgdipluls.NewProc("GdipSaveImageToFile")

	modole32            = syscall.NewLazyDLL("ole32.dll")
	procCLSIDFromString = modole32.NewProc("CLSIDFromString")
)

type EncoderParameter struct {
	Guid           winapi.GUID
	NumberOfValues uint32
	TypeAPI        uint32
	Value          uintptr
}

type EncoderParameters struct {
	Count     uint32
	Parameter [1]EncoderParameter
}

func CLSIDFromString(str *uint16) (clsid *winapi.GUID, err error) {
	var guid winapi.GUID
	err = nil

	hr, _, _ := syscall.Syscall(procCLSIDFromString.Addr(), 2, uintptr(unsafe.Pointer(str)), uintptr(unsafe.Pointer(&guid)), 0)
	if hr != 0 {
		err = syscall.Errno(hr)
	}

	clsid = &guid
	return
}

func GdipSaveImageToFile(image *winapi.GpBitmap, filename *uint16, clsidEncoder *winapi.GUID, encoderParams *EncoderParameters) winapi.GpStatus {
	ret, _, _ := syscall.Syscall6(procGdipSaveImageToFile.Addr(), 4, uintptr(unsafe.Pointer(image)), uintptr(unsafe.Pointer(filename)), uintptr(unsafe.Pointer(clsidEncoder)), uintptr(unsafe.Pointer(encoderParams)), 0, 0)
	return winapi.GpStatus(ret)
}

func savePNG(fileName string, newBMP []byte) error {
	var gdiplusStartupInput winapi.GdiplusStartupInput
	var gdiplusToken winapi.GdiplusStartupOutput

	gdiplusStartupInput.GdiplusVersion = 1
	if winapi.GdiplusStartup(&gdiplusStartupInput, &gdiplusToken) != 0 {
		return fmt.Errorf("failed to initialize GDI+")
	}
	defer winapi.GdiplusShutdown()

	var bmp *winapi.GpBitmap
	if r0, _, _ := procGdipCreateBitmapFromGdiDib.Call(
		uintptr(unsafe.Pointer(&newBMP[0])),
		uintptr(unsafe.Pointer(&newBMP[52])),
		uintptr(unsafe.Pointer(&bmp))); r0 != 0 {
		return fmt.Errorf("failed to create bitmap")
	}
	defer winapi.GdipDisposeImage((*winapi.GpImage)(bmp))
	sclsid, err := syscall.UTF16PtrFromString("{557CF406-1A04-11D3-9A73-0000F81EF32E}")
	if err != nil {
		return err
	}
	clsid, err := CLSIDFromString(sclsid)
	if err != nil {
		return err
	}
	fname, err := syscall.UTF16PtrFromString(fileName)
	if err != nil {
		return err
	}
	if GdipSaveImageToFile(bmp, fname, clsid, nil) != 0 {
		return fmt.Errorf("failed to call PNG encoder")
	}
	return nil
}

func main() {
	flag.Parse()

	winapi.OpenClipboard(0)
	defer winapi.CloseClipboard()

	names := flag.Args()
	if len(names) == 0 {
		names = []string{"clipboard.png"}
	}
	if hMem := winapi.GetClipboardData(winapi.CF_DIB); hMem != 0 {
		if lpBuff := winapi.GlobalLock(winapi.HGLOBAL(hMem)); lpBuff != nil {
			size, _, _ := procGlobalSize.Call(uintptr(hMem))
			ba := (*[1 << 32]byte)(unsafe.Pointer(lpBuff))[:size-10]
			for _, name := range names {
				if err := savePNG(name, ba[:]); err != nil {
					fmt.Fprintln(os.Stderr, os.Args[0]+":", err)
				}
			}
			winapi.GlobalUnlock(winapi.HGLOBAL(hMem))
		}
	} else {
		fmt.Fprintln(os.Stderr, os.Args[0]+": no bitmap")
	}
}
