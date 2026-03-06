#!/usr/bin/env node

/**
 * Extract and modify markdown document heading structure.
 * 
 * Commands:
 *   extract  - Output heading structure as JSON for LLM analysis
 *   apply    - Apply heading level changes from a plan file
 *   demote   - Batch demote headings (H2→H3, H3→H4, etc.)
 *   promote  - Batch promote headings (H3→H2, H4→H3, etc.)
 * 
 * Usage:
 *   node restructure-toc.mjs <directory> extract
 *   node restructure-toc.mjs <directory> apply --plan plan.json
 *   node restructure-toc.mjs <directory> demote --dry-run
 *   node restructure-toc.mjs <directory> promote --dry-run
 */

import fs from 'fs';
import path from 'path';

const args = process.argv.slice(2);
const dirArg = args.find(a => !a.startsWith('--') && !['extract', 'apply', 'demote', 'promote'].includes(a));
const command = args.find(a => ['extract', 'apply', 'demote', 'promote'].includes(a)) || 'extract';
const dryRun = args.includes('--dry-run');

const planIdx = args.indexOf('--plan');
const planFile = planIdx >= 0 ? args[planIdx + 1] : null;

if (!dirArg) {
  console.error('Usage: node restructure-toc.mjs <directory> <command> [options]');
  console.error('');
  console.error('Commands:');
  console.error('  extract        Output heading structure as JSON');
  console.error('  apply          Apply changes from a plan file');
  console.error('  demote         Batch demote headings (H2→H3, H3→H4)');
  console.error('  promote        Batch promote headings (H3→H2, H4→H3)');
  console.error('');
  console.error('Options:');
  console.error('  --dry-run      Preview changes without applying');
  console.error('  --plan <file>  JSON file with change plan');
  process.exit(1);
}

const targetDir = path.resolve(dirArg);
if (!fs.existsSync(targetDir)) {
  console.error(`Directory not found: ${targetDir}`);
  process.exit(1);
}

/**
 * Extract headings from content (skips fenced code blocks)
 */
function extractHeadings(content) {
  const lines = content.split('\n');
  const headings = [];
  let inCodeBlock = false;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (/^(`{3,}|~{3,})/.test(line)) {
      inCodeBlock = !inCodeBlock;
      continue;
    }
    if (inCodeBlock) continue;

    const match = line.match(/^(#{1,6})\s+(.+)$/);
    if (match) {
      headings.push({
        level: match[1].length,
        text: match[2].trim(),
        line: i + 1
      });
    }
  }
  return headings;
}

/**
 * Get all markdown files
 */
function getMdFiles(dir) {
  return fs.readdirSync(dir)
    .filter(f => f.endsWith('.md'))
    .sort();
}

// === EXTRACT COMMAND ===
if (command === 'extract') {
  const files = getMdFiles(targetDir);
  const result = [];
  
  for (const file of files) {
    const filepath = path.join(targetDir, file);
    const content = fs.readFileSync(filepath, 'utf-8');
    const headings = extractHeadings(content);
    
    result.push({
      file,
      headings
    });
  }
  
  console.log(JSON.stringify(result, null, 2));
}

// === APPLY COMMAND ===
else if (command === 'apply') {
  if (!planFile) {
    console.error('Error: --plan <file> is required for apply command');
    console.error('');
    console.error('Plan file format:');
    console.error(JSON.stringify({
      changes: [
        { file: "example.md", line: 10, fromLevel: 2, toLevel: 3 },
        { file: "example.md", line: 20, fromLevel: 2, toLevel: 3 }
      ]
    }, null, 2));
    process.exit(1);
  }
  
  if (!fs.existsSync(planFile)) {
    console.error(`Plan file not found: ${planFile}`);
    process.exit(1);
  }
  
  let plan;
  try {
    plan = JSON.parse(fs.readFileSync(planFile, 'utf-8'));
  } catch (e) {
    console.error(`Failed to parse plan file: ${e.message}`);
    process.exit(1);
  }
  
  // Group changes by file
  const changesByFile = {};
  for (const change of plan.changes) {
    if (!changesByFile[change.file]) {
      changesByFile[change.file] = [];
    }
    changesByFile[change.file].push(change);
  }
  
  // Apply changes
  for (const [file, changes] of Object.entries(changesByFile)) {
    const filepath = path.join(targetDir, file);
    if (!fs.existsSync(filepath)) {
      console.error(`File not found: ${filepath}`);
      continue;
    }
    
    const content = fs.readFileSync(filepath, 'utf-8');
    const lines = content.split('\n');
    
    // Sort changes by line (descending) to avoid offset issues
    changes.sort((a, b) => b.line - a.line);
    
    for (const change of changes) {
      const lineIdx = change.line - 1;
      if (lineIdx < 0 || lineIdx >= lines.length) {
        console.error(`Invalid line ${change.line} in ${file}`);
        continue;
      }
      
      const line = lines[lineIdx];
      const match = line.match(/^(#{1,6})(\s+.+)$/);
      if (!match) {
        console.error(`Line ${change.line} in ${file} is not a heading`);
        continue;
      }
      
      const currentLevel = match[1].length;
      if (currentLevel !== change.fromLevel) {
        console.error(`Line ${change.line} in ${file}: expected H${change.fromLevel}, found H${currentLevel}`);
        continue;
      }
      
      if (dryRun) {
        console.log(`${file}:${change.line} H${change.fromLevel}→H${change.toLevel}: ${match[2].trim()}`);
      } else {
        lines[lineIdx] = '#'.repeat(change.toLevel) + match[2];
      }
    }
    
    if (!dryRun) {
      fs.writeFileSync(filepath, lines.join('\n'));
      console.log(`✓ ${file}: ${changes.length} changes applied`);
    }
  }
  
  if (dryRun) {
    console.log('\n[DRY RUN] No changes applied');
  }
}

// === DEMOTE COMMAND ===
else if (command === 'demote') {
  const files = getMdFiles(targetDir);
  
  for (const file of files) {
    const filepath = path.join(targetDir, file);
    const content = fs.readFileSync(filepath, 'utf-8');
    const lines = content.split('\n');
    let changed = 0;
    let inCodeBlock = false;
    
    const newLines = lines.map(line => {
      if (/^(`{3,}|~{3,})/.test(line)) { inCodeBlock = !inCodeBlock; return line; }
      if (inCodeBlock) return line;
      // Process shallowest first: H1→H2, H2→H3, ..., H5→H6
      for (let from = 1; from <= 5; from++) {
        const pattern = new RegExp(`^#{${from}}(\\s+.+)$`);
        const match = line.match(pattern);
        if (match) {
          changed++;
          return '#'.repeat(from + 1) + match[1];
        }
      }
      return line;
    });
    
    if (dryRun) {
      const oldH = extractHeadings(content);
      console.log(`\n📄 ${file} (${changed} changes)`);
      for (const h of oldH.slice(0, 5)) {
        const newLevel = h.level < 6 ? h.level + 1 : h.level;
        console.log(`   H${h.level}→H${newLevel}: ${h.text}`);
      }
    } else {
      fs.writeFileSync(filepath, newLines.join('\n'));
      console.log(`✓ ${file}: ${changed} headings demoted`);
    }
  }
  
  if (dryRun) {
    console.log('\n[DRY RUN] No changes applied');
  }
}

// === PROMOTE COMMAND ===
else if (command === 'promote') {
  const files = getMdFiles(targetDir);
  
  for (const file of files) {
    const filepath = path.join(targetDir, file);
    const content = fs.readFileSync(filepath, 'utf-8');
    const lines = content.split('\n');
    let changed = 0;
    let inCodeBlock = false;
    
    const newLines = lines.map(line => {
      if (/^(`{3,}|~{3,})/.test(line)) { inCodeBlock = !inCodeBlock; return line; }
      if (inCodeBlock) return line;
      // Process deepest first: H6→H5, H5→H4, ..., H2→H1
      for (let from = 6; from >= 2; from--) {
        const pattern = new RegExp(`^#{${from}}(\\s+.+)$`);
        const match = line.match(pattern);
        if (match) {
          changed++;
          return '#'.repeat(from - 1) + match[1];
        }
      }
      return line;
    });
    
    if (dryRun) {
      const oldH = extractHeadings(content);
      console.log(`\n📄 ${file} (${changed} changes)`);
      for (const h of oldH.slice(0, 5)) {
        const newLevel = h.level > 1 ? h.level - 1 : h.level;
        console.log(`   H${h.level}→H${newLevel}: ${h.text}`);
      }
    } else {
      fs.writeFileSync(filepath, newLines.join('\n'));
      console.log(`✓ ${file}: ${changed} headings promoted`);
    }
  }
  
  if (dryRun) {
    console.log('\n[DRY RUN] No changes applied');
  }
}
