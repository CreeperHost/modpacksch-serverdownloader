package net.creeperhost.ftbserverdownloader;

import net.creeperhost.creeperlauncher.api.DownloadableFile;

import java.util.ArrayList;

public class ServerVersion {
    public long id;
    public long pack;
    public String name;
    public String type;
    private ArrayList<DownloadableFile> files = new ArrayList<DownloadableFile>();
    ServerVersion(long pack, long id)
    {
        this.pack = pack;
        this.id = id;
    }
    public void downloadManifest()
    {

    }
}
