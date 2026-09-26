function getById(id) {
   return document.getElementById(id);
}

function GetCenterPositionCode(width, height)
{
	isNetscape = navigator.appName.indexOf('Netscape')>=0;
	isExplorer = navigator.appName.indexOf('Explorer')>=0;

	sw  = screen.width;
	sh  = screen.height - 100;
	wbx = Math.round((sw-width)/2);
	wby = Math.round((sh-height)/2);
	
	if (isNetscape)
		return 'screenX='+wbx+',screenY='+wby;
	else
		return 'left='+wbx+',top='+wby;
}

function ActivateGame(gameId, buttonClientId, statClientId)
{
	var btnEnter = window.document.getElementById(buttonClientId);
	if (btnEnter != null)
		btnEnter.style.display = '';

	if (statClientId != '') {
		var lnkStat = window.document.getElementById(statClientId);
		if (lnkStat != null)
			lnkStat.style.display = '';
	}	
}
function OpenSearchWindow()
{
	var w = 410;
	var h = 300;
	newWin = window.open('/PlayerSearch.aspx', 'EnPlayersSearch', 'height='+h+',width='+w+',resizable=yes,scrollbars=yes,' + GetCenterPositionCode(w,h));
	newWin.focus();
}
function OpenerReload()
{
	opener.document.location.href = opener.document.location.href;
}

function WindowClose()
{
	window.close();
}

function OpenScrollableWindow(link, winname, width, height)
{
	var w = width;
	var h = height;
	cal = window.open(link, winname, 'toolbar=no,menubar=no,status=no,scrollbars=yes,resizable=1,height='+h+',width='+w+',' + GetCenterPositionCode(w,h));
	if (!cal.opener)
		cal.opener = self;
	cal.focus();
}

function OpenFixedSizeWindow(link, winname, width, height)
{
	var w = width;
	var h = height;
	cal = window.open(link, winname, 'toolbar=no,menubar=no,status=no,scrollbars=no,resizable=0,height='+h+',width='+w+',' + GetCenterPositionCode(w,h));
	if (!cal.opener)
		cal.opener = self;
	cal.focus();
}

function OpenNewPhotoGalleryWindow( link, winname )
{
	var w = 820;
	var h = 500; 
	OpenFixedSizeWindow( link, winname, w, h );
}

function OpenUserRanksWindow()
{
	var w = 680;
	var h = 490;
	OpenFixedSizeWindow( '/UserRanks.aspx', 'UserRanks', w, h );
}

function confirmClick( message )
{
	confirmed = window.confirm ( message );
	if (!confirmed)
	{
		event.returnValue = false;
		return false;
	}
}

//takes UTC date and returns array of offset sign and GMT offset value
//the first index is sign, the second is value
function GetGMTOffsetInfo(strDate) 
{
	var date = new Date();
	if (strDate != null)
		date = new Date(strDate);
		
	var offTime = date.getTimezoneOffset();
	var DST = 2*date.getTimezoneOffset() - new Date(date.getFullYear() - 1, 0).getTimezoneOffset() - new Date(date.getFullYear() - 1, 6).getTimezoneOffset();
	var offset = DST >= 0 ? -1*parseFloat(offTime / 60) : parseFloat((DST - offTime) / 60);
	//var offset = DST >= 0 ? parseFloat((DST - offTime) / 60) : -1*parseFloat(offTime / 60);
	var sign = offset > 0 ? "+" : offset < 0 ? "-" : "";
	
	return [sign, Math.abs(offset)];
}

//takes UTC date and returns array of offset sign and UTC offset value
//the first index is sign, the second is value
function GetUTCOffsetInfo(strDate) 
{
	var date = new Date();
	if (strDate != null)
		date = new Date(strDate);
		
	var offTime = date.getTimezoneOffset();
	var offset = -1*parseFloat(offTime / 60);
	var sign = offset > 0 ? "+" : offset < 0 ? "-" : "";
	
	return [sign, Math.abs(offset)];
}

//takes UTC date and returns local date
function DateToLocalString(strDate) {
	var date = new Date(strDate);
	return date.toLocaleString();
}

function FormatToLocalDate(date)
{ 
	var ms = new Array('01','02','03','04','05','06','07','08','09','10','11','12'); 
	var dt = date.getDate(); 
	var m = date.getMonth(); 
	var y = date.getYear(); 
	var h = date.getHours(); 
	var mm = date.getMinutes(); 
	if (y < 1000)
		y += 1900;
	if (mm < 10)
		mm = '0' + mm;
	if (h < 10)
		h = '0' + h; 
	
	return dt + '.' + ms[m] + '.' + y + '\,&#160;' + h + ':' + mm; 
}

function DisplayGameLocalTimeInfo(dateTime, timeCtrlID, utcCtrlID, gmtCtrlID)
{
	var ctrl1 = document.getElementById(timeCtrlID);
	var ctrl2 = document.getElementById(utcCtrlID);	
	var ctrl3 = document.getElementById(gmtCtrlID);
	if (ctrl1 != null)
	{
		ctrl1.innerHTML = DateToLocalString(dateTime);
	}
	if (ctrl2 != null)
	{
		//info = GetUTCOffsetInfo(dateTime);
		info = GetUTCOffsetInfo();
		ctrl2.innerHTML = info[0] + info[1];
	}
	if (ctrl3 != null)
	{
		//info = GetGMTOffsetInfo(dateTime);
		info = GetGMTOffsetInfo();
		ctrl3.innerHTML = info[0] + info[1];
	}
}

function GetGMT()
{
    var info = GetGMTOffsetInfo();
    return info[0] + info[1];	
}

function GetUTC() {
	var info = GetUTCOffsetInfo();
	return info[0] + info[1];
}

function write(cntId, text)
{
    $('#' + cntId).html(text);	
}

function CheckSectionName(errorMsg){
	var name = document.getElementsByName('TbSectionName')[0].value;
	if(name.length == 0) 
	{
		alert(errorMsg);
		return false;
	}
}

function GetLastDigit(val)
{
	val = val + "";
	return val.charAt(val.length-1);
}

function tests(timer)
{
	//alert(timer + '');
	//clearTimeout(timer);
}

var global = this;
function OnTick(timerID, codeToEval)
{
	//debugger;
	//alert(timerID + '');
	if (codeToEval != null && codeToEval != "void(0)")
	{
		eval('clearTimeout(' + timerID + ')');
		//global.eval(codeToEval + '(' + timerID + ')')
	}
}

// Sets the ramained time text using total number of seconds before time will be ended
function SetRemainedTimeText(n, ctrlWhereSet, timerID, lang, twoFormat, codeToEval){
	if (n <= 0) return;
	
	var timeSeconds = n - 1;
	SetTimeText(timeSeconds, lang, ctrlWhereSet, twoFormat);
	
	OnTick(timerID, codeToEval);
	
	if (timeSeconds > 0)
	{
		if (codeToEval != null)
			timerID = setTimeout(
							function () 
							{
								//SetRemainedTimeText(timeSeconds, document.getElementById(ctrlWhereSet.id), timerID, lang, twoFormat, codeToEval.replace(/\'/g, "\\\'"));
								SetRemainedTimeText(timeSeconds, document.getElementById(ctrlWhereSet.id), timerID, lang, twoFormat, 'tests');
							}, 
							1000);
		else
			timerID = setTimeout(
							function () 
							{
								SetRemainedTimeText(timeSeconds, document.getElementById(ctrlWhereSet.id), timerID, lang, twoFormat, 'tests');
							}, 
							1000);
	}
	else
	{
		eval('clearTimeout(' + timerID + ')');
		if (timeSeconds == 0)
		{	
			if (codeToEval != null) {				
				setTimeout('eval(\'' + codeToEval.replace(/\'/g, '\\\'') + '\');', 1000);
			} else {				
				setTimeout("location.href = location.href;", 1000);
			}
		}
	}
}

function SetNextTimeText(n, ctrlWhereSet, timerID, lang, twoFormat){
	if (n < 0) return;
	
	var dd, ddlong;
	var hh, hhlong;
	var mm, mmlong;
	var ss, sslong;
	
	var timeSeconds = n+1;
	dd = Math.floor(timeSeconds/86400);
	hh = Math.floor((timeSeconds - 86400*dd)/3600);
	mm = Math.floor((timeSeconds - 86400*dd - 3600*hh)/60);
	ss = timeSeconds - 86400*dd - 3600*hh - mm*60;
	var bold_start = "<b>";
	var bold_end = "</b>";
	if (lang == 'ru')
	{
		var last = GetLastDigit(dd);
		if (dd >= 11 && dd <= 19) { ddlong = self.strDay1; }
		else if (last == 1) { ddlong = self.strDay2; }
		else if (last >= 2 && last <= 4) { ddlong = self.strDay3; }
		else if (last >= 5 && last <= 9) { ddlong = self.strDay1; }
		else if (dd != 0 && last == 0) { ddlong = self.strDay1; }
		else ddlong = "";
		
		last = GetLastDigit(hh);
		if (hh >= 11 && hh <= 19) { hhlong = self.strHour1; }
		else if (last == 1) { hhlong = self.strHour2; }
		else if (last >= 2 && last <= 4) { hhlong = self.strHour3; }
		else if (last >= 5 && last <= 9) { hhlong = self.strHour1; }
		else if ((hh != 0 && last == 0) || (hh == 0 && dd != 0)) { hhlong = self.strHour1; }
		else hhlong = "";
		
		if ((twoFormat && dd <= 0) || !twoFormat)
		{
			last = GetLastDigit(mm);
			if (mm >= 11 && mm <= 19) { mmlong = self.strMinute1; }
			else if (last == 1) { mmlong = self.strMinute2; }
			else if (last >= 2 && last <= 4) { mmlong = self.strMinute3; }
			else if (last >= 5 && last <= 9) { mmlong = self.strMinute1; }
			else if ((mm != 0 && last == 0) || ((dd != 0 || hh != 0) && mm == 0)) { mmlong = self.strMinute1; }
			else mmlong = "";
		}
		else
			mmlong = "";
		
		if ((twoFormat && hh <= 0) || !twoFormat)
		{
			last = GetLastDigit(ss);
			if (ss >= 11 && ss <= 19) { sslong = self.strSecond1; }
			else if (last == 1) { sslong = self.strSecond2; }
			else if (last >= 2 && last <= 4) { sslong = self.strSecond3; }
			else if (last >= 5 && last <= 9) { sslong = self.strSecond1; }
			else if (last == 0) { sslong = self.strSecond1; }
			else sslong = "";
		}
		else
			sslong = "";
	}
	else
	{
		if (dd == 1) { ddlong = self.strDay3; }
		else if (dd > 1) { ddlong = self.strDay1; }
		else { ddlong = ""; }
	
		if (hh == 1) { hhlong = self.strHour2; }
		else if (hh > 1) { hhlong = self.strHour1; }
		else if (hh == 0 && dd != 0) { hhlong = self.strHour1; }
		else hhlong = "";
		
		if ((twoFormat && dd <= 0) || !twoFormat)
		{
			if (mm == 1) { mmlong = self.strMinute2; }
			else if (mm > 1) { mmlong = self.strMinute1; }
			else if (mm == 0 && (hh != 0 || dd != 0)) { mmlong = self.strMinute1; }
			else mmlong = "";
		}
		else
			mmlong = "";
		
		if ((twoFormat && hh <= 0) || !twoFormat)
		{	
			if (ss == 1) { sslong = self.strSecond2; }
			else if (ss > 1 || ss == 0) { sslong = self.strSecond1; }
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
				
	ctrlWhereSet.innerHTML = ddlong + hhlong + mmlong + sslong;
	
			
	if (timeSeconds > 0)
	{
		timerID = setTimeout(
							function ()
							{
								SetNextTimeText(timeSeconds, document.getElementById(ctrlWhereSet.id), timerID, lang, twoFormat);
							}, 
							1000);
	}
	else
	{
		eval('clearTimeout(' + timerID + ')');
		//if (timeSeconds == 0)
		//	location.href = location.href;
	}
}

function SetTimeText(timeSeconds, lang, ctrlWhereSet, twoFormat)
{
	var dd, ddlong;
	var hh, hhlong;
	var mm, mmlong;
	var ss, sslong;
	
	dd = Math.floor(timeSeconds/86400);
	hh = Math.floor((timeSeconds - 86400*dd)/3600);
	mm = Math.floor((timeSeconds - 86400*dd - 3600*hh)/60);
	ss = timeSeconds - 86400*dd - 3600*hh - mm*60;
	var bold_start = "<b>";
	var bold_end = "</b>";
	if (lang == 'ru')
	{
		var last = GetLastDigit(dd);
		if (dd >= 11 && dd <= 19) { ddlong = self.strDay1; }
		else if (last == 1) { ddlong = self.strDay2; }
		else if (last >= 2 && last <= 4) { ddlong = self.strDay3; }
		else if (last >= 5 && last <= 9) { ddlong = self.strDay1; }
		else if (dd != 0 && last == 0) { ddlong = self.strDay1; }
		else ddlong = "";
		
		last = GetLastDigit(hh);
		if (hh >= 11 && hh <= 19) { hhlong = self.strHour1; }
		else if (last == 1) { hhlong = self.strHour2; }
		else if (last >= 2 && last <= 4) { hhlong = self.strHour3; }
		else if (last >= 5 && last <= 9) { hhlong = self.strHour1; }
		else if ((hh != 0 && last == 0) || (hh == 0 && dd != 0)) { hhlong = self.strHour1; }
		else hhlong = "";
		
		if ((twoFormat && dd <= 0) || !twoFormat)
		{
			last = GetLastDigit(mm);
			if (mm >= 11 && mm <= 19) { mmlong = self.strMinute1; }
			else if (last == 1) { mmlong = self.strMinute2; }
			else if (last >= 2 && last <= 4) { mmlong = self.strMinute3; }
			else if (last >= 5 && last <= 9) { mmlong = self.strMinute1; }
			else if ((mm != 0 && last == 0) || ((dd != 0 || hh != 0) && mm == 0)) { mmlong = self.strMinute1; }
			else mmlong = "";
		}
		else
			mmlong = "";
		
		if ((twoFormat && hh <= 0) || !twoFormat)
		{
			last = GetLastDigit(ss);
			if (ss >= 11 && ss <= 19) { sslong = self.strSecond1; }
			else if (last == 1) { sslong = self.strSecond2; }
			else if (last >= 2 && last <= 4) { sslong = self.strSecond3; }
			else if (last >= 5 && last <= 9) { sslong = self.strSecond1; }
			else if (last == 0) { sslong = self.strSecond1; }
			else sslong = "";
		}
		else
			sslong = "";
	}
	else
	{
		if (dd == 1) { ddlong = self.strDay3; }
		else if (dd > 1) { ddlong = self.strDay1; }
		else { ddlong = ""; }
	
		if (hh == 1) { hhlong = self.strHour2; }
		else if (hh > 1) { hhlong = self.strHour1; }
		else if (hh == 0 && dd != 0) { hhlong = self.strHour1; }
		else hhlong = "";
		
		if ((twoFormat && dd <= 0) || !twoFormat)
		{
			if (mm == 1) { mmlong = self.strMinute2; }
			else if (mm > 1) { mmlong = self.strMinute1; }
			else if (mm == 0 && (hh != 0 || dd != 0)) { mmlong = self.strMinute1; }
			else mmlong = "";
		}
		else
			mmlong = "";
		
		if ((twoFormat && hh <= 0) || !twoFormat)
		{	
			if (ss == 1) { sslong = self.strSecond2; }
			else if (ss > 1 || ss == 0) { sslong = self.strSecond1; }
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
				
	ctrlWhereSet.innerHTML = ddlong + hhlong + mmlong + sslong;
}

function Reload()
{
	location.href = location.href;
}

function Stub() {}

if (typeof(String.prototype.ltrim) == 'undefined') {
  String.prototype.ltrim = function() {
    return this.replace(/^\s+/, '');
  }
}

if (typeof(String.prototype.rtrim) == 'undefined' ) {
  String.prototype.rtrim = function() {
    return this.replace(/\s+$/, '');
  }
}

if (typeof(String.prototype.trim) == 'undefined') {
  String.prototype.trim = function() {
    return this.replace(/^\s+/, '').replace(/\s+$/, '');
  }
}

function FormatCurrency(num, decimDel)
{
    num = num.toString().replace(/\$|\,/g,'');
    if (isNaN(num))
        num = "0";
    
    sign = (num == (num = Math.abs(num)));
    num = Math.floor(num * 100 + 0.50000000001);
    cents = num % 100;
    num = Math.floor(num / 100).toString();
    if (cents<10)
        cents = "0" + cents;
        
    for (var i = 0; i < Math.floor((num.length - (1 + i)) / 3); i++)
        num = num.substring(0, num.length - (4 * i + 3)) + ',' + num.substring(num.length - (4 * i + 3));
    return (((sign) ? '' : '-') + num + decimDel + cents);
}


function getElementsByNameFix(name, tagName) 
{
	var elem = document.getElementsByTagName(tagName);
	var arr = new Array();
	for(i = 0,iarr = 0; i < elem.length; i++) {
		att = elem[i].getAttribute("name");
		if(att == name) {
			arr[iarr] = elem[i];
			iarr++;
		}
	}
	return arr;
}



function getFirstChildElemByName(node, name)
{
	if (!node || !name) return null;

	if (node.attributes && node.attributes.name && node.attributes.name == name)
		return node;

	var children = node.childNodes

	for(var i=0;i<children.length; i++)
	{
		var child = children[i];
		if (!child)	continue;

		if (child.attributes && child.attributes.name && child.attributes.name.value == name)
			return child;

		child = getFirstChildElemByName(child, name);
		if (child)
			return child;
	}

	return null;
}


function SetFocusToFirstControl()
{
	var bFound = false;

	//for each form
	for (f=0; f < document.forms.length; f++) 
	{
		//for each element in each form
		for(i=0; i < document.forms[f].length; i++) 
		{
			//if it's not a hidden element
			if (document.forms[f][i].type != "hidden") 
			{
			//and it's not disabled
			if (document.forms[f][i].disabled != true) 
			{
				try {
					//set the focus to it
					document.forms[f][i].focus();
					var bFound = true;
				}
				catch(er) {
				}
			}
		}
		//if found in this element, stop looking
		if (bFound == true)
			break;
		}
		//if found in this form, stop looking
		if (bFound == true)
		  break;
	}
}

function ClearList(list)
{
	for (var i=list.options.length; i>0; i--)
	{
		list.options[i] = null;
	}
	list.options.length = 0;
}


function hide_email(contentHolderId, emailPart, withAt) {
	var cnt = document.getElementById(contentHolderId);
	cnt.innerHTML += emailPart;
	if (withAt) cnt.innerHTML += '@';
}

function escapeHTML (str)
{
   var div = document.createElement('div');
   var text = document.createTextNode(str);
   div.appendChild(text);   
   return div.innerHTML.replace(/\"/g, '&quot;');      
}

function ReloadImg(id)
{
	var i = $(id).attr('src');
	i = i.replace(/&rnd=.*$/ig, '');
	$(id).attr('src', i + '&rnd=' + Math.random());
	return false;
}

/**
* Bind function to context
* @param {Object|HTMLElement} context
* @param {Function} fn
* @return {Function} function bound to context
*/
function bindContext(context, fn /*args...*/) {
    if (typeof (context) == 'function') {
        var args = Array.prototype.slice.call(arguments, 0)
        args.unshift(null);
        return this.bind.apply(this, args);
    }

    if (arguments.length == 2) {    // params on call
        return function() {
            fn.apply(context || null, arguments);
        };
    } else {    // params on create
        var args = Array.prototype.slice.call(arguments, 2);
        return function() {
            fn.apply(context || null, args.concat(Array.prototype.slice.call(arguments, 0)));
        };
    }
}

function disableDblClick(name, timeout)
{      
	setTimeout(function(){ btnSwitch(name, false); }, timeout);
	setTimeout(function(){ btnSwitch(name, true); }, 1);
}

function btnSwitch(name, state){
    var b = document.getElementsByName(name);
    if (b.length > 0) 
        b[0].disabled=state;
}

function IsNullUndef(item) {
    return (typeof (item) == "undefined") || (item == null);
}

//GlobalOnlineHelp
function OnlineHelpInit()
{
    var seHFG1=document.createElement("script");
    seHFG1.type="text/javascript";
    var seHFG1s=(location.protocol.indexOf("https")==0?"https://secure.providesupport.com/image":"http://image.providesupport.com")+"/js/encounter/safe-standard.js?ps_h=HFG1\u0026ps_t="+new Date().getTime();
    setTimeout(function(){
        seHFG1.src=seHFG1s;
        document.getElementById('sdHFG1').appendChild(seHFG1);
        }, 1);
};

function OnlineHelpOnClick()
{
	document.getElementById('ifrProviderSupport').contentWindow.psrdPdow();
	return false;
}
function psHFG1ow(){OnlineHelpOnClick()};//поддержка прошлого названия функции

function AfterRndImgLoaded(params, resp)
{
    var img = params.cnt;
    if (!img || img.length < 1)
        return;
	var res = resp.split('\n');
	if (res.length > 0)
	{		
		img[0].src = res[0];
	}
}	

function getNewRndImage(imgId)
{
	if (!imgId) return;
	var img = $('#' + imgId);
	if (img.length == 0) return;
	AjaxHelper.get({ url: "/ALoader/RandomImage.aspx?c=" + img.src + "&rnd=" + Math.random(), cnt: img, success: AfterRndImgLoaded, wait: false })
	return false;
}
function Search(fromSearch, valueElemName, pageName) {
	this.fromSearch = fromSearch;
	this.valueElemName = valueElemName;
	this.pageName = pageName;
}
  
Search.prototype.SearchAll = function(eventClick)
{
	var sKey = document.getElementsByName('sKey')[0].value;
	var sValue = document.getElementsByName(this.valueElemName)[0].value;

	if ((IsNullUndef(sKey) || sKey.trim().length < 3) && (IsNullUndef(sValue) || sValue.trim().length < 3) && !this.eventClick) return;
	
	sText = encodeURIComponent(sValue).replace(/^\s+|\s+$/g,"");		
	sKey = sKey.replace(/^\s+|\s+$/g,"");			
	
	AjaxHelper.get({
		url: this.pageName + "?s=" + sKey + "&k=" + sValue,
		cnt: 'search',
		wait: (this.fromSearch) ? false : true,
		runJS: false});
	fromSearch = false;
}

function SetTitle(titleDelimeter) {
	$(document).ready(function() {
		var title = $('head > title');
		var titleText = title.text();
		if (titleText.indexOf(document.domain) < 0) {
			try { title.text(document.domain + titleDelimeter + titleText); }
			catch (err) { }
		}
	});
}

function moveToEditorAnchor()
{
	jQuery.ready(function () 
	{
		if(navigator.appName == "Microsoft Internet Explorer")
			window.scrollTo(0, $("#editor").offset().top);
		else 
			document.location = "#editor";
	});
}