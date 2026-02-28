function navModifierHeld(e) {
    if (e.metaKey) {
	return true; // common on macOS (command key)
    }
    if (e.ctrlKey) {
	return true; // common on Linux and Windows (ctrl)
    }
    return false;
}

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
