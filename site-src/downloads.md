---
layout: default
title: Downloads
---

# Downloads

## Current (v{{ site.data.current.version }})

{% if site.data.current.changes and site.data.current.changes.size > 0 %}
### Changes

{% for item in site.data.current.changes %}
- {{ item }}
{% endfor %}
{% else %}
<p class="muted">No current release notes available.</p>
{% endif %}

## All releases

<table>
  <thead>
    <tr>
      <th>Version</th>
      <th>Published</th>
      <th>Debian package</th>
      <th>Binary</th>
      <th>GitHub release</th>
      <th>Release Notes</th>
    </tr>
  </thead>
  <tbody>
    {% if site.data.releases and site.data.releases.size > 0 %}
      {% for rel in site.data.releases %}
      <tr>
        <td>{{ rel.version }}</td>
        <td>{{ rel.published }}</td>
        <td><a href="{{ rel.deb_url }}">.deb</a></td>
        <td><a href="{{ rel.binary_url }}">vaultline</a></td>
        <td><a href="{{ rel.release_url }}">GitHub</a></td>
        <td><a href="{{ rel.notes_url | relative_url }}">notes</a></td>
      </tr>
      {% endfor %}
    {% else %}
      <tr><td colspan="6" class="muted">No release entries yet. Run <code>./scripts/prepare-release.sh</code>.</td></tr>
    {% endif %}
  </tbody>
</table>
