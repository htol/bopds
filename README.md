# WIP: Basic OPD (bopds)

Trying to create simple OPDS server wich will be able to serve fb2 books.

# TODO:

 - [ ] Scan the library path to find out files (zip archives which include sevral .fb2 files inside)
 - [ ] Scan each archive to extract title/author from them
 - [ ] Map archive_name.zip/book_name.fb2 to title/author (probably in sqlite db at start)
 - [ ] Create search api
 - [ ] Create web server with OPDS api to serve requests an files
