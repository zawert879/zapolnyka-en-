package emu

import "testing"

// Образцы взяты с реальных страниц tech.en.cx (движок v1.88).
func TestRuDuration(t *testing.T) {
	cases := map[int]string{
		95: "1 минуту 35 секунд", 575: "9 минут 35 секунд", 58: "58 секунд", 298: "4 минуты 58 секунд",
		898: "14 минут 58 секунд", 3598: "59 минут 58 секунд", 60: "1 минуту", 120: "2 минуты", 300: "5 минут",
		30: "30 секунд", 180: "3 минуты", 900: "15 минут", 0: "0 секунд", 3600: "1 час", 7325: "2 часа 2 минуты 5 секунд",
		21: "21 секунду", 11: "11 секунд", 1500: "25 минут",
	}
	for sec, want := range cases {
		if got := ruDuration(sec); got != want {
			t.Errorf("ruDuration(%d) = %q, want %q", sec, got, want)
		}
	}
}

func TestPluralWords(t *testing.T) {
	for n, want := range map[int]string{1: "сектор", 2: "сектора", 4: "сектора", 5: "секторов", 6: "секторов", 7: "секторов", 11: "секторов", 12: "секторов", 21: "сектор"} {
		if got := sectorsWord(n); got != want {
			t.Errorf("sectorsWord(%d) = %q", n, got)
		}
	}
	for n, want := range map[int]string{1: "бонус", 2: "бонуса", 3: "бонуса", 6: "бонусов"} {
		if got := bonusesWord(n); got != want {
			t.Errorf("bonusesWord(%d) = %q", n, got)
		}
	}
}

func TestHTMLEncodeAndWrap(t *testing.T) {
	if got := HTMLEncode(`Т15 Стресс «кавычки» & <тег>`); got != "Т15 Стресс &#171;кавычки&#187; &amp; &lt;тег&gt;" {
		t.Errorf("HTMLEncode: %q", got)
	}
	if got := wrapLetters("неверный код"); got != "<i>неверный</i> <i>код</i>" {
		t.Errorf("wrapLetters: %q", got)
	}
	if got := wrapLetters("точка3"); got != "<i>точка</i>3" {
		t.Errorf("wrapLetters digits: %q", got)
	}
}

func TestTimerScript(t *testing.T) {
	got := timerScript("time1", 95, "/gameengines/encounter/play/82460/")
	want := "<span class=\"bold_off\" id=\"time1\">1 минуту 35 секунд</span><script type=\"text/javascript\">\r\n" +
		"\t//<![CDATA[\t\r\n" +
		"\twindow.time1 = new Timer({\"days\":[\"дн\",\"дн\",\"дн\"],\"hours\":[\"ч\",\"ч\",\"ч\"],\"minutes\":[\"м\",\"м\",\"м\"],\"seconds\":[\"с\",\"с\",\"с\"],\"StartCounter\":96,\"TimeDirection\":\"Down\",\"ShowTimeUnits\":true,\"TimerTextID\":\"time1\",\"DisplayTwoTimes\":false}, function(timer, seconds) { if (seconds > 0) return true; window.location = '/gameengines/encounter/play/82460/'; });\r\n" +
		"\t//]]>\r\n" +
		"\t</script>"
	if got != want {
		t.Errorf("timerScript:\n%q\nwant\n%q", got, want)
	}
}
