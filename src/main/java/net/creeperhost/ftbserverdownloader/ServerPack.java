package net.creeperhost.ftbserverdownloader;

import com.google.gson.Gson;
import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.ArrayList;
import java.util.concurrent.CompletableFuture;

public class ServerPack {
    public long id;
    public String name;
    public String synopsis;
    public String description;
    private ArrayList<CompletableFuture> futures = new ArrayList<>();
    public ArrayList<ServerVersion> versions = new ArrayList<ServerVersion>();
    ServerPack(long id)
    {
        this.id = id;
    }
    public void downloadManifest()
    {
        //Download and populate the pack and then we can return data!
        HttpClient wclient = HttpClient.newHttpClient();
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create("https://api.modpacks.ch/public/modpack/"+this.id))
                .build();
        wclient.sendAsync(request, HttpResponse.BodyHandlers.ofString())
                .thenApply(HttpResponse::body)
                .thenAccept((String data) -> {
                    Gson gson = new Gson();
                    JsonObject apiresp = gson.fromJson(data, JsonObject.class);
                    JsonArray versions = apiresp.getAsJsonArray("versions");
                    this.name = apiresp.get("name").getAsString();
                    this.synopsis = apiresp.get("synopsis").getAsString();
                    this.description = apiresp.get("description").getAsString();
                    for (JsonElement version : versions) {
                        JsonObject packVersion = version.getAsJsonObject();
                        long versionId = packVersion.get("id").getAsLong();
                        ServerVersion tmp = new ServerVersion(this.id, versionId);
                        futures.add(CompletableFuture.runAsync(() -> {
                            tmp.downloadManifest();
                        }));
                        this.versions.add(tmp);
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
}
