---
layout: default
title: Downloads
---

# Downloads

Latest known version from repository: **{{ site.vaultline_version }}**

<p class="muted">Release assets are loaded from GitHub Releases.</p>

<table>
  <thead>
    <tr>
      <th>Version</th>
      <th>Published</th>
      <th>Artifacts</th>
    </tr>
  </thead>
  <tbody id="release-table">
    <tr><td colspan="3" class="muted">Loading release metadata...</td></tr>
  </tbody>
</table>

<script>
(() => {
  const owner = {{ site.repo_owner | jsonify }};
  const repo = {{ site.repo_name | jsonify }};
  const tbody = document.getElementById('release-table');

  function releaseRow(release) {
    const assets = (release.assets || []).map(asset => {
      return `<a href="${asset.browser_download_url}">${asset.name}</a>`;
    }).join('<br>');

    const published = new Date(release.published_at || release.created_at).toISOString().slice(0, 10);
    return `<tr>
      <td><a href="${release.html_url}">${release.tag_name}</a></td>
      <td>${published}</td>
      <td>${assets || '<span class="muted">no assets</span>'}</td>
    </tr>`;
  }

  fetch(`https://api.github.com/repos/${owner}/${repo}/releases`)
    .then(res => {
      if (!res.ok) throw new Error(`GitHub API ${res.status}`);
      return res.json();
    })
    .then(releases => {
      if (!Array.isArray(releases) || releases.length === 0) {
        tbody.innerHTML = '<tr><td colspan="3" class="muted">No releases found.</td></tr>';
        return;
      }
      tbody.innerHTML = releases.map(releaseRow).join('');
    })
    .catch(err => {
      tbody.innerHTML = `<tr><td colspan="3" class="muted">Failed to load releases: ${err.message}</td></tr>`;
    });
})();
</script>

