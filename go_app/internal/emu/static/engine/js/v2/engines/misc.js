$(function () {


	$(".chat, .chatlabel, .chatinput").show();

	// History & Chat height
	historyPos = $("ul.history").offset();

	function HistoryChatResize() {
		var wh = $(window).height();
		var chatcontrols = $("#ChatForm").height() + 11;
		var indent = 20;
		if ($(".aside").hasClass("hasChat")) {
			$(".pane, #ChatFrame").css("height", (wh - historyPos.top - chatcontrols - indent) / 2);
		} else {
			$(".pane").css("height", (wh - historyPos.top - indent));
		}
	};

	HistoryChatResize();

	$(window).bind('resize', function () {
		HistoryChatResize();
	});


	// Select range fn
	$.fn.selectRange = function (start, end) {
		return this.each(function () {
			if (this.setSelectionRange) {
				this.focus();
				this.setSelectionRange(start, end);
			} else if (this.createTextRange) {
				var range = this.createTextRange();
				range.collapse(true);
				range.moveEnd('character', end);
				range.moveStart('character', start);
				range.select();
			}
		});
	};

	// jScrollPane
	$('.pane').each(function () {
		$(this).jScrollPane({
			showArrows: true,
			hideFocus: true,
			mouseWheelSpeed: 100,
			arrowButtonSpeed: 200,
			maintainPosition: true,
			stickToBottom: true,
			stickToRight: true,
			autoReinitialise: true,
			enableKeyboardNavigation: false,
			hideFocus: true
		});

		var api = $(this).data('jsp');
		var throttleTimeout;
		$(window).bind('resize', function () {
			if ($.browser.msie) {
				if (!throttleTimeout) {
					throttleTimeout = setTimeout(function () {
						api.reinitialise(); throttleTimeout = null;
					}, 50);
				}
			} else {
				api.reinitialise();
			}
		});
	});

	// Incorrect fading
	$("#incorrect").clone().prependTo("#incorrect").css({
		"color": "red",
		"position": "absolute",
		"top": "10",
		"left": "0",
		"background": "#1d1d1d"
	}).hide().fadeIn(200).delay(2000).fadeOut(500);


	$(".jspContainer").focus(function () {
		$(this).find(".jspVerticalBar").animate({
			opacity: 1
		}, 150);
	}).mouseenter(function () {
		$(this).find(".jspVerticalBar").animate({
			opacity: 1
		}, 150);
	}).mouseleave(function () {
		$(this).find(".jspVerticalBar").animate({
			opacity: 0
		}, 150);
	});


	$(document).ready(function () {

		if (ctrlToFocusId.length == 0)
			return;

		var $ctrlToFocus = $('#' + ctrlToFocusId);

		if ($ctrlToFocus.length == 0)
			return;

		var hint = $ctrlToFocus.attr("hint");

		if ($ctrlToFocus.val() != "" && $ctrlToFocus.val() != hint)
			$ctrlToFocus.selectRange(0, $ctrlToFocus.val().length);
		else
			$ctrlToFocus.selectRange(0, 0);
		
		
	});

});