package pkg

import (
	"strings"
)

// List of valid ISO 639-1 language codes
var validLanguageCodes = map[string]bool{
	"aa": true, "ab": true, "af": true, "ak": true, "sq": true, "am": true, "ar": true, "an": true, "hy": true, "as": true,
	"av": true, "ae": true, "ay": true, "az": true, "ba": true, "bm": true, "eu": true, "be": true, "bn": true, "bh": true,
	"bi": true, "bs": true, "br": true, "bg": true, "my": true, "ca": true, "ch": true, "ce": true, "zh": true, "cu": true,
	"cv": true, "kw": true, "co": true, "cr": true, "cs": true, "da": true, "dv": true, "nl": true, "dz": true, "en": true,
	"eo": true, "et": true, "ee": true, "fo": true, "fj": true, "fi": true, "fr": true, "fy": true, "ff": true, "gd": true,
	"gl": true, "lg": true, "ka": true, "de": true, "el": true, "kl": true, "gn": true, "gu": true, "ht": true, "ha": true,
	"he": true, "hz": true, "hi": true, "ho": true, "hu": true, "is": true, "io": true, "ig": true, "id": true, "ia": true,
	"ie": true, "iu": true, "ik": true, "ga": true, "it": true, "ja": true, "jv": true, "kn": true, "kr": true, "ks": true,
	"kk": true, "km": true, "ki": true, "rw": true, "ky": true, "kv": true, "kg": true, "ko": true, "kj": true, "ku": true,
	"lo": true, "la": true, "lv": true, "li": true, "ln": true, "lt": true, "lu": true, "lb": true, "mk": true, "mg": true,
	"ms": true, "ml": true, "mt": true, "gv": true, "mi": true, "mr": true, "mh": true, "mn": true, "na": true, "nv": true,
	"nd": true, "nr": true, "ng": true, "ne": true, "no": true, "nb": true, "nn": true, "ii": true, "oc": true, "oj": true,
	"or": true, "om": true, "os": true, "pi": true, "ps": true, "fa": true, "pl": true, "pt": true, "pa": true, "qu": true,
	"ro": true, "rm": true, "rn": true, "ru": true, "se": true, "sm": true, "sg": true, "sa": true, "sc": true, "sr": true,
	"sn": true, "sd": true, "si": true, "sk": true, "sl": true, "so": true, "st": true, "es": true, "su": true, "sw": true,
	"ss": true, "sv": true, "tl": true, "ty": true, "tg": true, "ta": true, "tt": true, "te": true, "th": true, "bo": true,
	"ti": true, "to": true, "ts": true, "tn": true, "tr": true, "tk": true, "tw": true, "ug": true, "uk": true, "ur": true,
	"uz": true, "ve": true, "vi": true, "vo": true, "wa": true, "cy": true, "wo": true, "xh": true, "yi": true, "yo": true,
	"za": true, "zu": true,
}

// Function to validate language code
func isValidLanguageCode(code string) bool {
	return validLanguageCodes[strings.ToLower(code)]
}
