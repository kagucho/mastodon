if (window.navigator.registerProtocolHandler) {
  document.addEventListener('DOMContentLoaded', () => {
    [].forEach.call(document.querySelectorAll('.remote-follow .button'), (content) => {
      content.href = document.getElementById('account_follow_intent').textContent;
    });
  });
}
