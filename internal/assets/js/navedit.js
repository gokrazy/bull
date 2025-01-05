function navEditModifierHeld(e) {
    if (e.metaKey) {
	return true; // common on macOS (command key)
    }
    if (e.ctrlKey) {
	return true; // common on Linux and Windows (ctrl)
    }
    return false;
}

const bullsave = document.getElementById('bull-save');

document.addEventListener('keydown', function(e) {
    if (!navEditModifierHeld(e)) {
	return;
    }
    if (e.key == 's') {
	// C-s (save)
	event.preventDefault();
	bullsave.click();
    }
});

