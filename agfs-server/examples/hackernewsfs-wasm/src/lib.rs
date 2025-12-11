//! HackerNewsFS WASM - Filesystem plugin that fetches Hacker News stories
//!
//! Provides access to Hacker News front page stories as markdown files
//! - cat /hackernews/refresh - Refreshes the story list
//! - ls /hackernews/frontpage/ - Lists all stories
//! - cat /hackernews/frontpage/1.md - Read a specific story

use agfs_wasm_ffi::prelude::*;
use indoc::formatdoc;
use serde::{Deserialize, Serialize};
use std::cell::RefCell;

const HN_API_BASE: &str = "https://hacker-news.firebaseio.com/v0";
const MAX_STORIES: usize = 30;

#[derive(Debug, Serialize, Deserialize)]
struct HNItem {
    id: u64,
    #[serde(default)]
    title: String,
    #[serde(default)]
    by: String,
    #[serde(default)]
    score: i64,
    #[serde(default)]
    url: String,
    #[serde(default)]
    text: String,
    #[serde(default)]
    descendants: i64,
    #[serde(default)]
    time: i64,
}

#[derive(Default)]
pub struct HackerNewsFS {
    stories: RefCell<Vec<HNItem>>,
}

impl HackerNewsFS {
    fn fetch_top_stories(&self) -> Result<()> {
        // Fetch top story IDs
        eprintln!("Fetching from: {}/topstories.json", HN_API_BASE);
        let response = Http::get(&format!("{}/topstories.json", HN_API_BASE))?;

        eprintln!("Response status: {}", response.status_code);
        eprintln!("Response headers: {:?}", response.headers);
        eprintln!("Response body length: {}", response.body.len());

        if !response.is_success() {
            return Err(Error::Other(format!("Failed to fetch top stories: HTTP {}", response.status_code)));
        }

        // Debug: print first 200 bytes of response
        let preview = if response.body.len() > 200 {
            String::from_utf8_lossy(&response.body[..200])
        } else {
            String::from_utf8_lossy(&response.body)
        };
        eprintln!("Response preview: '{}'", preview);
        eprintln!("Response first 20 bytes (hex): {:02x?}", &response.body[..response.body.len().min(20)]);

        if response.body.is_empty() {
            return Err(Error::Other("Response body is empty".to_string()));
        }

        let story_ids: Vec<u64> = response.json()
            .map_err(|e| Error::Other(format!("Failed to parse story IDs: {}", e)))?;

        // Fetch first MAX_STORIES items
        let mut stories = Vec::new();
        for (i, &id) in story_ids.iter().take(MAX_STORIES).enumerate() {
            match self.fetch_story(id) {
                Ok(story) => {
                    stories.push(story);
                }
                Err(e) => {
                    eprintln!("Failed to fetch story {}: {:?}", id, e);
                    // Continue with other stories
                }
            }

            // Show progress
            if (i + 1) % 10 == 0 {
                eprintln!("Fetched {}/{} stories...", i + 1, MAX_STORIES);
            }
        }

        *self.stories.borrow_mut() = stories;
        Ok(())
    }

    fn fetch_story(&self, id: u64) -> Result<HNItem> {
        let url = format!("{}/item/{}.json", HN_API_BASE, id);
        let response = Http::get(&url)?;

        if !response.is_success() {
            return Err(Error::Other(format!("HTTP {}", response.status_code)));
        }

        response.json()
            .map_err(|e| Error::Other(format!("Failed to parse story: {}", e)))
    }

    fn story_to_markdown(&self, index: usize, story: &HNItem) -> String {
        let url_line = if !story.url.is_empty() {
            format!("- **URL**: {}\n", story.url)
        } else {
            String::new()
        };

        let content_section = if !story.text.is_empty() {
            formatdoc! {"

                ## Content

                {}
            ", story.text}
        } else {
            String::new()
        };

        formatdoc! {"
            # {}

            **Story #{}**

            - **Author**: {}
            - **Score**: {}
            - **Comments**: {}
            - **ID**: {}
            {}- **Time**: {}
            {}
            ---
            View on HN: https://news.ycombinator.com/item?id={}
        ",
            story.title,
            index + 1,
            story.by,
            story.score,
            story.descendants,
            story.id,
            url_line,
            story.time,
            content_section,
            story.id
        }
    }
}

impl FileSystem for HackerNewsFS {
    fn name(&self) -> &str {
        "hackernewsfs"
    }

    fn readme(&self) -> &str {
        "HackerNewsFS - Access Hacker News stories as files\n\
         \n\
         Usage:\n\
         - cat /hackernews/refresh - Refresh story list from HN\n\
         - ls /hackernews/frontpage/ - List all stories\n\
         - cat /hackernews/frontpage/1.md - Read story #1\n\
         - cat /hackernews/frontpage/2.md - Read story #2\n\
         etc.\n"
    }

    fn config_params(&self) -> Vec<ConfigParameter> {
        vec![
            ConfigParameter::new(
                "max_stories",
                "int",
                false,
                "30",
                "Maximum number of stories to fetch"
            ),
        ]
    }

    fn initialize(&mut self, _config: &Config) -> Result<()> {
        // Fetch stories on initialization
        eprintln!("HackerNewsFS: Fetching initial stories...");
        self.fetch_top_stories()?;
        eprintln!("HackerNewsFS: Loaded {} stories", self.stories.borrow().len());
        Ok(())
    }

    fn read(&self, path: &str, _offset: i64, _size: i64) -> Result<Vec<u8>> {
        match path {
            "/refresh" => {
                // Trigger refresh
                self.fetch_top_stories()?;
                let msg = format!("Refreshed {} stories from Hacker News\n", self.stories.borrow().len());
                Ok(msg.into_bytes())
            }
            p if p.starts_with("/frontpage/") && p.ends_with(".md") => {
                // Extract story number from filename
                let filename = p.strip_prefix("/frontpage/")
                    .unwrap()
                    .strip_suffix(".md")
                    .unwrap();

                let index: usize = filename.parse()
                    .map_err(|_| Error::NotFound)?;

                if index == 0 || index > self.stories.borrow().len() {
                    return Err(Error::NotFound);
                }

                let stories = self.stories.borrow();
                let story = &stories[index - 1];
                let content = self.story_to_markdown(index - 1, story);
                Ok(content.into_bytes())
            }
            _ => Err(Error::NotFound),
        }
    }

    fn stat(&self, path: &str) -> Result<FileInfo> {
        match path {
            "/" => Ok(FileInfo::dir("hackernews", 0o755)),
            "/refresh" => {
                Ok(FileInfo::file("refresh", 0, 0o644))
            }
            "/frontpage" => {
                Ok(FileInfo::dir("frontpage", 0o755))
            }
            p if p.starts_with("/frontpage/") && p.ends_with(".md") => {
                let filename = p.strip_prefix("/frontpage/")
                    .unwrap()
                    .strip_suffix(".md")
                    .unwrap();

                let index: usize = filename.parse()
                    .map_err(|_| Error::NotFound)?;

                if index == 0 || index > self.stories.borrow().len() {
                    return Err(Error::NotFound);
                }

                let stories = self.stories.borrow();
                let story = &stories[index - 1];
                let content = self.story_to_markdown(index - 1, story);
                let name = format!("{}.md", index);

                Ok(FileInfo::file(&name, content.len() as i64, 0o644))
            }
            _ => Err(Error::NotFound),
        }
    }

    fn readdir(&self, path: &str) -> Result<Vec<FileInfo>> {
        match path {
            "/" => {
                Ok(vec![
                    FileInfo::file("refresh", 0, 0o644),
                    FileInfo::dir("frontpage", 0o755),
                ])
            }
            "/frontpage" => {
                let stories = self.stories.borrow();
                let mut entries = Vec::new();

                for (i, story) in stories.iter().enumerate() {
                    let name = format!("{}.md", i + 1);
                    let content = self.story_to_markdown(i, story);
                    entries.push(FileInfo::file(&name, content.len() as i64, 0o644));
                }

                Ok(entries)
            }
            _ => Err(Error::NotFound),
        }
    }

    fn write(&mut self, path: &str, _data: &[u8], _offset: i64, _flags: WriteFlag) -> Result<i64> {
        if path == "/refresh" {
            // Allow writing to refresh to trigger update
            self.fetch_top_stories()?;
            let msg = format!("Refreshed {} stories from Hacker News\n", self.stories.borrow().len());
            Ok(msg.len() as i64)
        } else {
            Err(Error::PermissionDenied)
        }
    }

    fn create(&mut self, _path: &str) -> Result<()> {
        Err(Error::PermissionDenied)
    }

    fn mkdir(&mut self, _path: &str, _perm: u32) -> Result<()> {
        Err(Error::PermissionDenied)
    }

    fn remove(&mut self, _path: &str) -> Result<()> {
        Err(Error::PermissionDenied)
    }

    fn remove_all(&mut self, _path: &str) -> Result<()> {
        Err(Error::PermissionDenied)
    }

    fn rename(&mut self, _old_path: &str, _new_path: &str) -> Result<()> {
        Err(Error::PermissionDenied)
    }

    fn chmod(&mut self, _path: &str, _mode: u32) -> Result<()> {
        Ok(())
    }
}

export_plugin!(HackerNewsFS);
