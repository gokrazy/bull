function hiddenElement(name, value) {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = name;
    input.value = value;
    return input;
}

function itaskclick(event) {
    const checkbox = this;
    const line = checkbox.dataset.line;
    console.log('itasklist item click (line ', line, ') on ', checkbox);
    const form = checkbox.closest('form');
    form.appendChild(hiddenElement('checkbox-line', line));
    form.submit();
}

const itasklists = document.getElementsByClassName('itasklist');
for (itasklist of itasklists) {
    const inputs = itasklist.getElementsByTagName('input');
    for (input of inputs) {
	if (input.type !== 'checkbox') {
	    continue;
	}
	input.addEventListener('click', itaskclick);
    }
}
