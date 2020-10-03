package net.creeperhost.modpackserverdownloader;

import com.google.gson.Gson;
import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import net.creeperhost.creeperlauncher.api.DownloadableFile;
import net.creeperhost.creeperlauncher.util.FileUtils;
import net.creeperhost.creeperlauncher.util.MiscUtils;

import java.io.File;
import java.io.FileWriter;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.ArrayList;
import java.util.concurrent.CompletableFuture;

public class ServerVersion {
    public long id;
    public long pack;
    public String name;
    public String type;
    public String Modloader;
    public String ModloaderType;
    public long minimumRam;
    public long recommendRam;
    public String Vanilla;
    private ArrayList<CompletableFuture> futures = new ArrayList<>();
    private ArrayList<DownloadableFile> files = new ArrayList<DownloadableFile>();
    ServerVersion(long pack, long id)
    {
        this.pack = pack;
        this.id = id;
    }
    public void downloadManifest()
    {
        HttpClient wclient = HttpClient.newHttpClient();
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create("https://api.modpacks.ch/public/modpack/"+this.pack+"/"+this.id))
                .build();
        wclient.sendAsync(request, HttpResponse.BodyHandlers.ofString())
                .thenApply(HttpResponse::body)
                .thenAccept((String data) -> {
                    Gson gson = new Gson();
                    JsonObject apiresp = gson.fromJson(data, JsonObject.class);
                    JsonArray files = apiresp.getAsJsonArray("files");
                    JsonArray targets = apiresp.getAsJsonArray("targets");
                    this.name = apiresp.get("name").getAsString();
                    this.type = apiresp.get("type").getAsString();
                    JsonObject specs = apiresp.getAsJsonObject("specs");
                    this.minimumRam = specs.get("minimum").getAsLong();
                    this.recommendRam = specs.get("recommended").getAsLong();
                    for(JsonElement target : targets)
                    {
                        JsonObject tar = target.getAsJsonObject();
                        if(tar.get("type").getAsString().equals("game"))
                        {
                            this.Vanilla = tar.get("version").getAsString();
                        }
                        if(tar.get("type").getAsString().equals("modloader"))
                        {
                            this.Modloader = tar.get("version").getAsString();
                            this.ModloaderType = tar.get("name").getAsString();
                        }
                    }

                    for (JsonElement file : files) {
                        JsonObject fileInfo = file.getAsJsonObject();
                        DownloadableFile downloadableFile = new DownloadableFile(
                                fileInfo.get("version").getAsString(),
                                fileInfo.get("path").getAsString(),
                                fileInfo.get("url").getAsString(),
                                new ArrayList<>(){{add(fileInfo.get("sha1").getAsString());}},
                                fileInfo.get("size").getAsLong(),
                                fileInfo.get("clientonly").getAsBoolean(),
                                fileInfo.get("optional").getAsBoolean(),
                                fileInfo.get("id").getAsLong(),
                                fileInfo.get("name").getAsString(),
                                fileInfo.get("type").getAsString(),
                                fileInfo.get("updated").getAsString());
                        this.files.add(downloadableFile);
                    }
                }).join();
        CompletableFuture<Void> combinedFuture = CompletableFuture.allOf(
                futures.toArray(new CompletableFuture[0])).exceptionally((t) ->
                {
                    t.printStackTrace();
                    return null;
                }
        );
        combinedFuture.join();
    }
    public void install()
    {
        ArrayList<CompletableFuture<Void>> futures = new ArrayList<>();

        boolean modloaderDownloading = false;

        String installerFileName = "";

        if (this.ModloaderType.equals("forge")) {
            URI forgeURI = MiscUtils.findForgeDownloadURL(this.Vanilla, this.Modloader);
            if (forgeURI != null) {
                try {
                    String fileName = forgeURI.toString().substring(forgeURI.toString().lastIndexOf('/') + 1);
                    DownloadableFile downloadableFile = new DownloadableFile(this.Modloader, "./", forgeURI.toString(), null, -1, false, false, -1, fileName, "modloader", "0");
                    // TODO: Actual checksum and sizes
                    installerFileName = downloadableFile.getName();
                    files.add(downloadableFile);
                    modloaderDownloading = true;
                } catch (Exception e) {

                }
            }
        }

        files.add(new DownloadableFile("version", "./", "https://api.modpacks.ch/public/modpack/"+this.pack+"/"+this.id, null, -1, false, false, -1, "version.json", "modloader", "0")); // download version

        File mods = new File(Main.installPath.toFile(), "mods/");
        File coremods = new File(Main.installPath.toFile(), "coremods/");
        File instmods = new File(Main.installPath.toFile(), "instmods/");

        File config = new File(Main.installPath.toFile(), "config/");
        File resources = new File(Main.installPath.toFile(), "resources/");
        File scripts = new File(Main.installPath.toFile(), "scripts/");

        FileUtils.deleteDirectory(mods);
        FileUtils.deleteDirectory(coremods);
        FileUtils.deleteDirectory(instmods);
        FileUtils.deleteDirectory(config);
        FileUtils.deleteDirectory(resources);
        FileUtils.deleteDirectory(scripts);


        ArrayList<String> directories = new ArrayList<String>();
        for(DownloadableFile downloadableFile : files) {
            if (!downloadableFile.getClientOnly()) {
                //Just feels too dangerous in case someone makes a mistake in the db.
                /*if(!directories.contains(downloadableFile.getPath()))
                {
                    if(downloadableFile.getPath().length() > 2) {
                        directories.add(downloadableFile.getPath());
                        File dr = Main.installPath.resolve(downloadableFile.getPath()).toFile();
                        if(dr.exists()) {
                           FileUtils.deleteDirectory(dr);
                        }
                    }
                }*/
                futures.add(CompletableFuture.runAsync(() -> {
                    try {
                        downloadableFile.prepare();
                        downloadableFile.download(Main.installPath.resolve(downloadableFile.getPath()).resolve(downloadableFile.getName()), true, false);
                    } catch (Throwable throwable) {
                        System.out.println("[" + Main.dlnum.get() + "/" + files.size() + "] Unable to download: " + throwable.getMessage());
                        throwable.printStackTrace();
                    }
                }).thenRunAsync(() -> {
                    System.out.println("[" + Main.dlnum.incrementAndGet() + "/" + files.size() + "] Downloaded '" + downloadableFile.getName() + "' to '" + downloadableFile.getPath() + "' [" + downloadableFile.getSize() + " bytes]...");
                }));
            }
        }

        CompletableFuture<Void> combinedFuture = CompletableFuture.allOf(
                futures.toArray(new CompletableFuture[0])).exceptionally((t) ->
                {
                    t.printStackTrace();
                    return null;
                }
        );
        combinedFuture.join();

        System.out.println("["+Main.dlnum.get()+"/"+files.size()+"] Finished.");
        try {
            HttpClient wclient = HttpClient.newHttpClient();
            HttpRequest request = HttpRequest.newBuilder()
                    .uri(URI.create("https://api.modpacks.ch/public/modpack/" + this.pack + "/" + this.id))
                    .setHeader("User-Agent", "ModpackServerDownloader/"+Main.verString)
                    .build();
            wclient.sendAsync(request, HttpResponse.BodyHandlers.ofString())
                    .thenApply(HttpResponse::body)
                    .thenAccept((String data) -> {
                        //Not sure we'd ever wanna do anything with this anyway.
                    }).join();
        } catch (Exception ignored) {}//Nobody cares if our analytics fail... Not even us.
        if (ModloaderType.equals("forge")) {
            if (modloaderDownloading)
            {
                ProcessBuilder processBuilder = new ProcessBuilder().command("java", "-jar", Main.installPath.resolve(installerFileName).toAbsolutePath().toString(), "--installServer", Main.installPath.toAbsolutePath().toString());
                processBuilder.directory(Main.installPath.toFile());
                processBuilder.inheritIO();
                boolean error = false;
                try {
                    System.out.println("Invoking forge " + Modloader + " installer.");
                    System.out.println("================= FORGE INSTALL BEGINS =================");
                    Process start = processBuilder.start();
                    start.waitFor();
                } catch (IOException | InterruptedException e) {
                    error = true;
                }
                if(!(Main.installPath.resolveSibling("forge-" + Vanilla + "-" + Modloader + ".jar").toFile().exists() || Main.installPath.resolveSibling("forge-" + Vanilla + "-" + Modloader + "-universal.jar").toFile().exists())) {
                    error = true;
                }

                System.out.println("=================  FORGE INSTALL ENDS  =================");

                if (error) {
                    //System.out.println("An error occurred whilst installing forge " + Modloader + ". Please install manually.");
                } else {
                    Main.installPath.resolveSibling(installerFileName + ".log").toFile().delete();
                    Main.installPath.resolveSibling(installerFileName).toFile().delete();
                }
            } else {
                System.out.println("Could not find download location for forge " + Modloader + ". Please install manually.");
            }
        } else {
            System.out.println("Modloader type " + ModloaderType + " is not currently supported. Please check if an updated version of the downloader is available.");
        }
        if(Main.generateStart) {
            String forgeJar = "";
            if (Main.installPath.resolveSibling("forge-" + Vanilla + "-" + Modloader + ".jar").toFile().exists()) {
                forgeJar = "forge-" + Vanilla + "-" + Modloader + ".jar";
            }
            if (Main.installPath.resolveSibling("forge-" + Vanilla + "-" + Modloader + "-universal.jar").toFile().exists()) {
                forgeJar = "forge-" + Vanilla + "-" + Modloader + "-universal.jar";
            }
            String startCmd = "-server -XX:+UseG1GC -XX:+UnlockExperimentalVMOptions -Xmx" + this.recommendRam + "M -Xms" + this.minimumRam + "M -jar "+forgeJar+" nogui";
            File bash = new File(Main.installPath.resolve("start.sh").toAbsolutePath().toString());
            try {
                if (bash.createNewFile()) {
                    String bashFile = "" +
                            "#!/bin/bash\n" +
                            "if [ -r eula.txt ] && grep -qF eula=true eula.txt ; then\n" +
                            "    echo \"EULA already accepted. Starting Minecraft...\"\n" +
                            "else\n" +
                            "    echo \"Do you agree to the Mojang EULA available at https://account.mojang.com/documents/minecraft_eula ?\"\n" +
                            "    read -r -p \"[y/n] \" EULA\n" +
                            "    if [ \"$EULA\" = \"y\" ]; then\n" +
                            "        echo \"eula=true\" > eula.txt\n" +
                            "    fi\n" +
                            "fi\n" +
                            "java " + startCmd + "\n" +
                            "if [ 0 -ne $? ] ; then\n" +
                            "    echo 'Failed. Printing some information for toubleshooting...'\n" +
                            "    java -version\n" +
                            "fi\n" +
                            "read -r -n 1 -p \"Press any key to exit\"";
                    FileWriter bashWriter = new FileWriter(bash.toPath().toAbsolutePath().toString());
                    bashWriter.write(bashFile);
                    bashWriter.close();
                    bash.setExecutable(true, true);
                }
            } catch (IOException e) {
                e.printStackTrace();
            }
            File batch = new File(Main.installPath.resolve("start.bat").toAbsolutePath().toString());
            try {
                if (batch.createNewFile()) {
                    // BatCodeCheck displays false warning for indented echo lines and warning about /p is harmless because reading only one character
                    String batchFile = "" +
                            "@echo off\r\n" +
                            "find \"eula=true\" eula.txt 1 > NUL 2>&1\r\n" +
                            "if %ERRORLEVEL% EQU 0 (\r\n" +
                            "   echo \"EULA already accepted. Starting Minecraft...\"\r\n" +
                            "   GOTO END\r\n" +
                            ")\r\n" +
                            "echo \"Do you agree to the Mojang EULA available at https://account.mojang.com/documents/minecraft_eula ?\"\r\n" +
                            "set /p EULA=[y/n]\r\n" +
                            "IF /I \"%EULA%\" NEQ \"y\" GOTO END\r\n" +
                            "echo eula=true>eula.txt\r\n" +
                            ":END\r\n" +
                            "java.exe " + startCmd + "\r\n" +
                            "if %ERRORLEVEL% NEQ 0 (\r\n" +
                            "   echo \"Failed. Printing some information for toubleshooting...\"\r\n" +
                            "   java -version\r\n" +
                            ")\r\n" +
                            "pause\r\n";
                    FileWriter batchWriter = new FileWriter(batch.toPath().toAbsolutePath().toString());
                    batchWriter.write(batchFile);
                    batchWriter.close();
                }
            } catch (IOException e) {
                e.printStackTrace();
            }
        }
        System.out.println("Pack install Finished.");
    }
}
