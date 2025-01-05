function navModifierHeld(e) {
    if (e.metaKey) {
	return true; // common on macOS (command key)
    }
    if (e.ctrlKey) {
	return true; // common on Linux and Windows (ctrl)
    }
    return false;
}

const navedit = document.getElementById('bull_nav_edit');
const navmostrecent = document.getElementById('bull_nav_mostrecent');
const navsearch = document.getElementById('bull_nav_search');
const navindex = document.getElementById('bull_nav_index');

document.addEventListener('keydown', function(e) {
    if (!navModifierHeld(e)) {
	return;
    }
    if (e.key == 'e') {
	// C-e (_e_dit)
	event.preventDefault();
	navedit.click();
    }
    if (e.key == 'm') {
	// C-m (_m_ost recent)
	event.preventDefault();
	navmostrecent.click();
    }
    if (e.key == 'k') {
	// C-k (search), as C-s conflicts with save in edit view
	event.preventDefault();
	navsearch.click();
    }
    if (e.key == 'i') {
	// C-i (index)
	event.preventDefault();
	navindex.click();
    }
});

