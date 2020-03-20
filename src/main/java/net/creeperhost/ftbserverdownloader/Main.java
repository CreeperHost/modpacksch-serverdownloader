package net.creeperhost.ftbserverdownloader;

import net.creeperhost.creeperlauncher.CreeperLogger;
import net.creeperhost.creeperlauncher.api.DownloadableFile;
import net.creeperhost.creeperlauncher.install.tasks.DownloadTask;

import java.io.File;
import java.net.MalformedURLException;
import java.net.URI;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.atomic.AtomicLong;

public class Main {
    public static AtomicLong overallBytes;
    public static AtomicLong currentBytes;

    public static void main(String[] args) {
	// write your code here
    }

    void downloadFiles(File instanceDir, File forgeLibs)
    {
        CreeperLogger.INSTANCE.info("Attempting to downloaded required files");

        ArrayList<CompletableFuture> futures = new ArrayList<>();
        overallBytes.set(0);

        currentBytes.set(0);

        List<DownloadableFile> requiredFiles = null;
        try
        {
            requiredFiles = getRequiredDownloads(new File(instanceDir + File.separator + "version.json"), forgeLibs);
        } catch (MalformedURLException err)
        {
            err.printStackTrace();
            return;
        }
        //Need to loop first for overallBytes or things get weird.
        for (DownloadableFile file : requiredFiles)
        {
            Path path = Paths.get(file.getPath());
            if (!path.toFile().exists())
            {
                if (file.getSize() > 0)
                {
                    overallBytes.addAndGet(file.getSize());
                }
            }
        }
        for (DownloadableFile file : requiredFiles)
        {
            File f = new File(instanceDir + File.separator + file.getPath());
            if (!f.exists()) f.mkdir();
            try
            {
                URI url = new URI(file.getUrl());
                Path path = Paths.get(file.getPath());
                if (!path.toFile().exists())
                {
                    DownloadTask task = new DownloadTask(file, path);//url, path, file.getSize(), false, file.getSha1() );
                    futures.add(task.execute());
                }
            } catch (Exception e)
            {
                e.printStackTrace();
            }
        }
        try
        {
            CompletableFuture<Void> combinedFuture = CompletableFuture.allOf(
                    futures.toArray(new CompletableFuture[0])).exceptionally((t) ->
                    {
                        t.printStackTrace();
                        return null;
                    }
            );

            futures.forEach((blah) ->
            {
                ((CompletableFuture<Void>) blah).exceptionally((t) ->
                {
                    combinedFuture.completeExceptionally(t);
                    return null;
                });
            });

            combinedFuture.join();

        } catch (Throwable err)
        {
            CreeperLogger.INSTANCE.error(err.getMessage());
            for (CompletableFuture ftr : futures)
            {
                ftr.cancel(true);
            }
            throw err;
        }
    }

    private List<DownloadableFile> getRequiredDownloads(File file, File forgeLibs) throws MalformedURLException {
        // This is where you give stuffs
        return null;
    }
}
