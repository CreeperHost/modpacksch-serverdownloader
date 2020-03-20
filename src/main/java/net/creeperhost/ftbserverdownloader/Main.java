package net.creeperhost.ftbserverdownloader;

import com.google.gson.Gson;
import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import net.creeperhost.creeperlauncher.CreeperLogger;
import net.creeperhost.creeperlauncher.api.DownloadableFile;
import net.creeperhost.creeperlauncher.install.tasks.DownloadTask;

import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLParameters;
import java.io.File;
import java.io.IOException;
import java.io.UnsupportedEncodingException;
import java.net.*;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.Executor;
import java.util.concurrent.atomic.AtomicLong;

public class Main {
    public static AtomicLong overallBytes;
    public static AtomicLong currentBytes;
    private static ArrayList<ServerPack> packs = new ArrayList<ServerPack>();
    public static void main(String[] args) {
        boolean search = false;
        boolean latest = true;
	    try
        {
            //Do we have an pack ID or are we searching for a pack?
            Long.parseLong(args[0]);
        } catch(Exception ignored)
        {
            search = true;
        }
	    if(!search) {
            if (args.length > 1) {
                try {
                    //Do we have a version ID or are we just grabbing the latest?
                    Long.parseLong(args[1]);
                } catch (Exception ignored) {
                    search = true;
                }
            }
        }
	    if(search)
        {
            String term = null;
            try {
                term = URLEncoder.encode(args[0], "UTF-8");
            } catch (UnsupportedEncodingException e) {
                e.printStackTrace();
            }
            ArrayList<CompletableFuture> futures = new ArrayList<>();
            HttpClient wclient = HttpClient.newHttpClient();
            HttpRequest request = HttpRequest.newBuilder()
                        .uri(URI.create("https://api.modpacks.ch/public/modpack/search/8?term="+term))
                        .build();
            wclient.sendAsync(request, HttpResponse.BodyHandlers.ofString())
                    .thenApply(HttpResponse::body)
                    .thenAccept((String data) -> {
                        Gson gson = new Gson();
                        JsonObject apiresp = gson.fromJson(data, JsonObject.class);
                        JsonArray packs = apiresp.getAsJsonArray("packs");
                        for(JsonElement pack : packs)
                        {
                            long packId = pack.getAsLong();
                            ServerPack tmp = new ServerPack(packId);
                            futures.add(CompletableFuture.runAsync(() -> {
                               tmp.downloadManifest();
                            }));
                        }
                    });
            CompletableFuture<Void> combinedFuture = CompletableFuture.allOf(
                    futures.toArray(new CompletableFuture[0])).exceptionally((t) ->
                    {
                        t.printStackTrace();
                        return null;
                    }
            );
            combinedFuture.join();

        }
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

    public static String getDefaultThreadLimit(String arg)
    {
        return String.valueOf((Runtime.getRuntime().availableProcessors() / 2) - 1);
    }

    private List<DownloadableFile> getRequiredDownloads(File file, File forgeLibs) throws MalformedURLException {
        // This is where you give stuffs
        return null;
    }
}
