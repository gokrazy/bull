function navModifierHeld(e) {
    if (e.metaKey) {
	return true; // common on macOS (command key)
    }
    if (e.ctrlKey) {
	return true; // common on Linux and Windows (ctrl)
    }
    return false;
}

// Add relative timestamps ("5m ago") to <time> elements in the browse table.
(function() {
    const now = new Date();
    const times = document.querySelectorAll('.bull_gen_browse time[datetime]');
    for (let i = 0; i < times.length; i++) {
	const el = times[i];
	const dt = new Date(el.getAttribute('datetime'));
	const ago = now - dt;
	if (Math.abs(ago) < 5000) {
	    el.textContent += ' • just now';
	    continue;
	}
	if (ago < 0 || ago >= 24 * 60 * 60 * 1000) {
	    continue;
	}
	const seconds = Math.floor(ago / 1000);
	const minutes = Math.floor(seconds / 60);
	const hours = Math.floor(minutes / 60);
	let text;
	if (hours > 0) {
	    text = hours + 'h ' + (minutes % 60) + 'm ago';
	} else if (minutes > 0) {
	    text = minutes + 'm ' + (seconds % 60) + 's ago';
	} else {
	    text = seconds + 's ago';
	}
	el.textContent += ' • ' + text;
    }
})();

const rows = document.querySelectorAll('.bull_gen_browse tbody tr');
var selected = 0;
if (rows.length > 0) {
    rows[0].classList.add('bull_browse_selected');
}

function browseSelect(idx) {
    rows[selected].classList.remove('bull_browse_selected');
    selected = idx;
    rows[selected].classList.add('bull_browse_selected');
    rows[selected].scrollIntoView({block: 'nearest'});
}

document.addEventListener('keydown', function(e) {
    if (e.target.tagName === 'INPUT' ||
	e.target.tagName === 'TEXTAREA') {
	return;
    }
    if (navModifierHeld(e) || e.altKey) {
	return;
    }
    if (e.key === 'j' && selected < rows.length - 1) {
	e.preventDefault();
	browseSelect(selected + 1);
    }
    if (e.key === 'k' && selected > 0) {
	e.preventDefault();
	browseSelect(selected - 1);
    }
    if (e.key === 'Enter' && document.activeElement === document.body) {
	e.preventDefault();
	const link = rows[selected].querySelector('a');
	link.click();
    }
});
