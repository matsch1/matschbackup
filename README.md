**WORK IN PROGRESS**

This is my personal backup software.
I use it to automatically backup some specific files to my NAS.

It relies on a preconfigured **rclone** config, which in my case enables a smb connection to my Fritz!NAS.

I use an autostart script, which checks already existing backups and creates new backups if necessary.

## Features
- Create backups of provided path list
- Zip compression of directories before upload
- Deletion of old backups if a maximum number of backups is reached.
