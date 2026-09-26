function Timer(params, callback) {
    this.FireStep = 1000;
    jQuery.extend(true, this, params);
    this.Callback = callback;    
    this.StartTime = new Date().getTime();
    this.TimerID = setTimeout(bindContext(this, this.Tick), 0);
	 this.Stop = false;
    return this;
}

Timer.prototype =
{
	Tick: function() {
		var t = this; var needTick = true; var currentSeconds;
		var correctionTime = parseInt((new Date().getTime() - t.StartTime) / 1000);
		if (t.TimeDirection == 'Down') {
			currentSeconds = t.StartCounter - correctionTime;
			needTick = currentSeconds > 0;
			if (!needTick)
			{ currentSeconds = 0; }
		}
		else {
			currentSeconds = t.StartCounter + correctionTime;

			if (!t.ShowTimeUnits) {
				if (GetGMTOffsetInfo()[0] == "-")
				{ currentSeconds += -(GetGMTOffsetInfo()[1] * 3600); }
				else
				{ currentSeconds += GetGMTOffsetInfo()[1] * 3600; }
			}
		}

		this.SetTimeTextEx(currentSeconds, 'ru', document.getElementById(t.TimerTextID),
				t.DisplayTwoTimes, t.days, t.hours,
				t.minutes, t.seconds);


		var go = true;

		if (typeof (t.Callback) != "undefined") {
			try {
				if (typeof (t.callbackParams) != "undefined")
					go = t.Callback(t, currentSeconds, t.callbackParams);
				else
					(go = t.Callback(t, currentSeconds));
			} catch (e) { }
		}

		if (go && needTick)
		{ t.TimerID = setTimeout(bindContext(this, this.Tick), t.FireStep); }
	},

	Clear: function () {
		clearTimeout(this.TimerID);
	},

	SetTimeTextEx: function(timeSeconds, lang, ctrlWhereSet, twoFormat, days, hours, minutes, seconds) {
		var dd, ddlong;
		var hh, hhlong;
		var mm, mmlong;
		var ss, sslong;

		dd = Math.floor(timeSeconds / 86400);
		hh = Math.floor((timeSeconds - 86400 * dd) / 3600);
		mm = Math.floor((timeSeconds - 86400 * dd - 3600 * hh) / 60);
		ss = timeSeconds - 86400 * dd - 3600 * hh - mm * 60;
		var bold_start = "<b>";
		var bold_end = "</b>";
		if (lang == 'ru') {
			var last = this.GetLastDigit(dd);
			if (dd >= 11 && dd <= 19) { ddlong = days[0]; }
			else if (last == 1) { ddlong = days[1]; }
			else if (last >= 2 && last <= 4) { ddlong = days[2]; }
			else if (last >= 5 && last <= 9) { ddlong = days[0]; }
			else if (dd != 0 && last == 0) { ddlong = days[0]; }
			else ddlong = "";

			last = this.GetLastDigit(hh);
			if (hh >= 11 && hh <= 19) { hhlong = hours[0]; }
			else if (last == 1) { hhlong = hours[1]; }
			else if (last >= 2 && last <= 4) { hhlong = hours[2]; }
			else if (last >= 5 && last <= 9) { hhlong = hours[0]; }
			else if ((hh != 0 && last == 0) || (hh == 0 && dd != 0)) { hhlong = hours[0]; }
			else hhlong = "";

			if ((twoFormat && dd <= 0) || !twoFormat) {
				last = this.GetLastDigit(mm);
				if (mm >= 11 && mm <= 19) { mmlong = minutes[0]; }
				else if (last == 1) { mmlong = minutes[1]; }
				else if (last >= 2 && last <= 4) { mmlong = minutes[2]; }
				else if (last >= 5 && last <= 9) { mmlong = minutes[0]; }
				else if ((mm != 0 && last == 0) || ((dd != 0 || hh != 0) && mm == 0)) { mmlong = minutes[0]; }
				else mmlong = "";
			}
			else
				mmlong = "";

			if ((twoFormat && hh <= 0 && dd <= 0) || !twoFormat) {
				last = this.GetLastDigit(ss);
				if (ss >= 11 && ss <= 19) { sslong = seconds[0]; }
				else if (last == 1) { sslong = seconds[1]; }
				else if (last >= 2 && last <= 4) { sslong = seconds[2]; }
				else if (last >= 5 && last <= 9) { sslong = seconds[0]; }
				else if (last == 0) { sslong = seconds[0]; }
				else sslong = "";
			}
			else
				sslong = "";
		}
		else {
			if (dd == 1) { ddlong = days[2]; }
			else if (dd > 1) { ddlong = days[0]; }
			else { ddlong = ""; }

			if (hh == 1) { hhlong = hours[1]; }
			else if (hh > 1) { hhlong = hours[0]; }
			else if (hh == 0 && dd != 0) { hhlong = hours[0]; }
			else hhlong = "";

			if ((twoFormat && dd <= 0) || !twoFormat) {
				if (mm == 1) { mmlong = minutes[1]; }
				else if (mm > 1) { mmlong = minutes[0]; }
				else if (mm == 0 && (hh != 0 || dd != 0)) { mmlong = minutes[0]; }
				else mmlong = "";
			}
			else
				mmlong = "";

			if ((twoFormat && hh <= 0 && dd <= 0) || !twoFormat) {
				if (ss == 1) { sslong = seconds[1]; }
				else if (ss > 1 || ss == 0) { sslong = seconds[0]; }
				else sslong = "";
			}
			else
				sslong = "";
		}

		if (ddlong != "")
			ddlong = bold_start + dd + bold_end + " " + ddlong + " ";
		if (hhlong != "")
			hhlong = bold_start + hh + bold_end + " " + hhlong + " ";
		if (mmlong != "")
			mmlong = bold_start + mm + bold_end + " " + mmlong + " ";
		if (sslong != "")
			sslong = bold_start + ss + bold_end + " " + sslong;


		if (minutes[0] == ":") {
			ddlong = "";
			hhlong = hh + ":";

			if (mm < 10)
				mmlong = "0" + mm + ":";
			else
				mmlong = mm + ":";

			if (ss < 10)
				sslong = "0" + ss;
			else
				sslong = ss;
		}

		ctrlWhereSet.innerHTML = ddlong + hhlong + mmlong + sslong;
	},

	GetLastDigit: function(val) {
		val = val + "";
		return val.charAt(val.length - 1);
	}
}

function TickHandlerStub(timer, seconds) {    
	return true;
}

function OnFireTimerTick(timer, seconds) {
    if (typeof (timerVisible) == 'undefined' || !enPanelId) return true;
    if (timerVisible == false && seconds <= 15 * 60) {
        obj = document.getElementById(enPanelId + '_lblGameError');
        if (obj != null && obj.style.display != 'none')
            obj.style.display = 'none';

        obj = document.getElementById(enPanelId + '_TimerHolder');
        if (obj != null && obj.style.display == 'none')
            obj.style.display = '';

        timerVisible = true;
    }
    else if (seconds <= 0) {
        window.location.reload();
        return false;
    }
    return true;
}

function OnGameEnterTimerTick(timer, seconds, params) {
    jQuery.extend(true, this, params);    
    var EnterGameTimerHolder = $('#' + this.EnterGameTimerHolder);
	if (seconds <= 15*60)
	{
	    var divEnterGameHolder = $('#' + this.divEnterGameHolder);
	    divEnterGameHolder.show();	
	    if (remainTimeVisible)	
		    EnterGameTimerHolder.show()
	}
	
	if (seconds <= 0)
	{
	    EnterGameTimerHolder.hide();
			
		var divStatisticLinkHolder = $("#" + this.divStatisticLinkHolder);
		if (this.statVisible)
		    divStatisticLinkHolder.show();
	}		
	return true;
}

function RefreshWindow(timer, seconds) {
	if (seconds <= 0) {
		location.href = window.timerLocationReload;
		return false;
	}
	return true;
}

$(function () {

	$.timerDefUnits = { "days": ["", "", ""], "hours": ["", "", ""], "minutes": ["", "", ""], "seconds": ["", "", ""] };

	$.addTimer = function (elId, start, options, callback) {
		var defOpts = jQuery.extend($.timerDefUnits, { "TimeDirection": "Down", "ShowTimeUnits": true, "DisplayTwoTimes": false});

		var mergedOpts = jQuery.extend(defOpts, options || {}, { 'TimerTextID': elId, "StartCounter": start });
		var call = function (timer, seconds) {
			if (seconds > 0) return true;
			if (typeof (callback) != 'undefined')
				callback(timer);
		};

		var timers = jQuery.data(document, 'timers') || [];
		timers.push(new Timer(mergedOpts, call));
		jQuery.data(document, 'timers', timers);
	};
	$.getAllTimers = function () {
		return jQuery.data(document, 'timers') || [];
	};

	$.removeAllTimers = function () {
		var timers = jQuery.data(document, 'timers') || [];
		jQuery.removeData(document, 'timers');

		for (var i in timers) {
			timers[i].Clear();
			delete timers[i];
		}
		delete timers;
	};
});